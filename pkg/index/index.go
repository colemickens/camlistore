/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package index

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/context"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/sorted"
	"camlistore.org/pkg/strutil"
	"camlistore.org/pkg/types"
	"camlistore.org/pkg/types/camtypes"
)

type Index struct {
	*blobserver.NoImplStorage

	s sorted.KeyValue

	KeyFetcher blob.StreamingFetcher // for verifying claims

	// BlobSource is used for fetching blobs when indexing files and other
	// blobs types that reference other objects.
	BlobSource blobserver.FetcherEnumerator

	// deletes is a cache to keep track of the deletion status (deleted vs undeleted)
	// of the blobs in the index. It makes for faster reads than the otherwise
	// recursive calls on the index.
	deletes *deletionCache

	corpus *Corpus // or nil, if not being kept in memory
}

var (
	_ blobserver.Storage = (*Index)(nil)
	_ Interface          = (*Index)(nil)
)

var aboutToReindex = false

// SetImpendingReindex notes that the user ran the camlistored binary with the --reindex flag.
// Because the index is about to be wiped, schema version checks should be suppressed.
func SetImpendingReindex() {
	// TODO: remove this function, once we refactor how indexes are created.
	// They'll probably not all have their own storage constructor registered.
	aboutToReindex = true
}

// New returns a new index using the provided key/value storage implementation.
func New(s sorted.KeyValue) *Index {
	idx := &Index{s: s}
	if aboutToReindex {
		idx.deletes = &deletionCache{
			m: make(map[blob.Ref][]deletion),
		}
		return idx
	}

	schemaVersion := idx.schemaVersion()
	switch {
	case schemaVersion == 0 && idx.isEmpty():
		// New index.
		err := idx.s.Set(keySchemaVersion.name, fmt.Sprintf("%d", requiredSchemaVersion))
		if err != nil {
			panic(fmt.Sprintf("Could not write index schema version %q: %v", requiredSchemaVersion, err))
		}
	case schemaVersion != requiredSchemaVersion:
		tip := ""
		if os.Getenv("CAMLI_DEV_CAMLI_ROOT") != "" {
			// Good signal that we're using the devcam server, so help out
			// the user with a more useful tip:
			tip = `(For the dev server, run "devcam server --wipe" to wipe both your blobs and index)`
		} else {
			tip = "Run 'camlistored --reindex' (it might take awhile, but shows status). Alternative: 'camtool dbinit' (or just delete the file for a file based index), and then 'camtool sync --all'"
		}
		log.Fatalf("index schema version is %d; required one is %d. You need to reindex. %s",
			schemaVersion, requiredSchemaVersion, tip)
	}
	if err := idx.initDeletesCache(); err != nil {
		panic(fmt.Sprintf("Could not initialize index's deletes cache: %v", err))
	}
	return idx
}

func (x *Index) isEmpty() bool {
	iter := x.s.Find("", "")
	hasRows := iter.Next()
	if err := iter.Close(); err != nil {
		panic(err)
	}
	return !hasRows
}

func (x *Index) Reindex() error {
	ctx := context.TODO()

	wiper, ok := x.s.(sorted.Wiper)
	if !ok {
		return fmt.Errorf("index's storage type %T doesn't support sorted.Wiper", x.s)
	}
	log.Printf("Wiping index storage type %T ...", x.s)
	if err := wiper.Wipe(); err != nil {
		return fmt.Errorf("error wiping index's sorted key/value type %T: %v", x.s, err)
	}
	log.Printf("Index wiped. Rebuilding...")

	reindexStart, _ := blob.Parse(os.Getenv("CAMLI_REINDEX_START"))

	err := x.s.Set(keySchemaVersion.name, fmt.Sprintf("%d", requiredSchemaVersion))
	if err != nil {
		return err
	}

	var nerrmu sync.Mutex
	nerr := 0

	blobc := make(chan blob.Ref, 32)

	enumCtx := ctx.New()
	enumErr := make(chan error, 1)
	go func() {
		defer close(blobc)
		donec := enumCtx.Done()
		var lastTick time.Time
		enumErr <- blobserver.EnumerateAll(enumCtx, x.BlobSource, func(sb blob.SizedRef) error {
			now := time.Now()
			if lastTick.Before(now.Add(-1 * time.Second)) {
				log.Printf("Reindexing at %v", sb.Ref)
				lastTick = now
			}
			if reindexStart.Valid() && sb.Ref.Less(reindexStart) {
				return nil
			}
			select {
			case <-donec:
				return context.ErrCanceled
			case blobc <- sb.Ref:
				return nil
			}
		})
	}()
	const j = 4 // arbitrary concurrency level
	var wg sync.WaitGroup
	for i := 0; i < j; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for br := range blobc {
				if err := x.reindex(br); err != nil {
					log.Printf("Error reindexing %v: %v", br, err)
					nerrmu.Lock()
					nerr++
					nerrmu.Unlock()
					// TODO: flag (or default?) to stop the EnumerateAll above once
					// there's any error with reindexing?
				}
			}
		}()
	}
	if err := <-enumErr; err != nil {
		return err
	}
	wg.Wait()

	log.Printf("Index rebuild complete.")
	nerrmu.Lock() // no need to unlock
	if nerr != 0 {
		return fmt.Errorf("%d blobs failed to re-index", nerr)
	}
	if err := x.initDeletesCache(); err != nil {
		return err
	}
	return nil
}

func queryPrefixString(s sorted.KeyValue, prefix string) sorted.Iterator {
	if prefix == "" {
		return s.Find("", "")
	}
	lastByte := prefix[len(prefix)-1]
	if lastByte == 0xff {
		panic("unsupported query prefix ending in 0xff")
	}
	end := prefix[:len(prefix)-1] + string(lastByte+1)
	return s.Find(prefix, end)
}

func (x *Index) queryPrefixString(prefix string) sorted.Iterator {
	return queryPrefixString(x.s, prefix)
}

func queryPrefix(s sorted.KeyValue, key *keyType, args ...interface{}) sorted.Iterator {
	return queryPrefixString(s, key.Prefix(args...))
}

func (x *Index) queryPrefix(key *keyType, args ...interface{}) sorted.Iterator {
	return x.queryPrefixString(key.Prefix(args...))
}

func closeIterator(it sorted.Iterator, perr *error) {
	err := it.Close()
	if err != nil && *perr == nil {
		*perr = err
	}
}

// schemaVersion returns the version of schema as it is found
// in the currently used index. If not found, it returns 0.
func (x *Index) schemaVersion() int {
	schemaVersionStr, err := x.s.Get(keySchemaVersion.name)
	if err != nil {
		if err == sorted.ErrNotFound {
			return 0
		}
		panic(fmt.Sprintf("Could not get index schema version: %v", err))
	}
	schemaVersion, err := strconv.Atoi(schemaVersionStr)
	if err != nil {
		panic(fmt.Sprintf("Bogus index schema version: %q", schemaVersionStr))
	}
	return schemaVersion
}

type deletion struct {
	deleter blob.Ref
	when    time.Time
}

type byDeletionDate []deletion

func (d byDeletionDate) Len() int           { return len(d) }
func (d byDeletionDate) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d byDeletionDate) Less(i, j int) bool { return d[i].when.Before(d[j].when) }

type deletionCache struct {
	sync.RWMutex
	m map[blob.Ref][]deletion
}

// initDeletesCache creates and populates the deletion status cache used by the index
// for faster calls to IsDeleted and DeletedAt. It is called by New.
func (x *Index) initDeletesCache() error {
	x.deletes = &deletionCache{
		m: make(map[blob.Ref][]deletion),
	}
	var err error
	it := x.queryPrefix(keyDeleted)
	defer closeIterator(it, &err)
	for it.Next() {
		cl, ok := kvDeleted(it.Key())
		if !ok {
			return fmt.Errorf("Bogus keyDeleted entry key: want |\"deleted\"|<deleted blobref>|<reverse claimdate>|<deleter claim>|, got %q", it.Key())
		}
		targetDeletions := append(x.deletes.m[cl.Target],
			deletion{
				deleter: cl.BlobRef,
				when:    cl.Date,
			})
		sort.Sort(sort.Reverse(byDeletionDate(targetDeletions)))
		x.deletes.m[cl.Target] = targetDeletions
	}
	return err
}

func kvDeleted(k string) (c camtypes.Claim, ok bool) {
	// TODO(bradfitz): garbage
	keyPart := strings.Split(k, "|")
	if len(keyPart) != 4 {
		return
	}
	if keyPart[0] != "deleted" {
		return
	}
	target, ok := blob.Parse(keyPart[1])
	if !ok {
		return
	}
	claimRef, ok := blob.Parse(keyPart[3])
	if !ok {
		return
	}
	date, err := time.Parse(time.RFC3339, unreverseTimeString(keyPart[2]))
	if err != nil {
		return
	}
	return camtypes.Claim{
		BlobRef: claimRef,
		Target:  target,
		Date:    date,
		Type:    string(schema.DeleteClaim),
	}, true
}

// IsDeleted reports whether the provided blobref (of a permanode or
// claim) should be considered deleted.
func (x *Index) IsDeleted(br blob.Ref) bool {
	if x.deletes == nil {
		// We still allow the slow path, in case someone creates
		// their own Index without a deletes cache.
		return x.isDeletedNoCache(br)
	}
	x.deletes.RLock()
	defer x.deletes.RUnlock()
	return x.isDeleted(br)
}

// The caller must hold x.deletes.mu for read.
func (x *Index) isDeleted(br blob.Ref) bool {
	deletes, ok := x.deletes.m[br]
	if !ok {
		return false
	}
	for _, v := range deletes {
		if !x.isDeleted(v.deleter) {
			return true
		}
	}
	return false
}

// Used when the Index has no deletes cache (x.deletes is nil).
func (x *Index) isDeletedNoCache(br blob.Ref) bool {
	var err error
	it := x.queryPrefix(keyDeleted, br)
	for it.Next() {
		cl, ok := kvDeleted(it.Key())
		if !ok {
			panic(fmt.Sprintf("Bogus keyDeleted entry key: want |\"deleted\"|<deleted blobref>|<reverse claimdate>|<deleter claim>|, got %q", it.Key()))
		}
		if !x.isDeletedNoCache(cl.BlobRef) {
			closeIterator(it, &err)
			if err != nil {
				// TODO: Do better?
				panic(fmt.Sprintf("Could not close iterator on keyDeleted: %v", err))
			}
			return true
		}
	}
	closeIterator(it, &err)
	if err != nil {
		// TODO: Do better?
		panic(fmt.Sprintf("Could not close iterator on keyDeleted: %v", err))
	}
	return false
}

// GetRecentPermanodes sends results to dest filtered by owner, limit, and
// before.  A zero value for before will default to the current time.  The
// results will have duplicates supressed, with most recent permanode
// returned.
// Note, permanodes more recent than before will still be fetched from the
// index then skipped. This means runtime scales linearly with the number of
// nodes more recent than before.
func (x *Index) GetRecentPermanodes(dest chan<- camtypes.RecentPermanode, owner blob.Ref, limit int, before time.Time) (err error) {
	defer close(dest)

	keyId, err := x.KeyId(owner)
	if err == sorted.ErrNotFound {
		log.Printf("No recent permanodes because keyId for owner %v not found", owner)
		return nil
	}
	if err != nil {
		log.Printf("Error fetching keyId for owner %v: %v", owner, err)
		return err
	}

	sent := 0
	var seenPermanode dupSkipper

	if before.IsZero() {
		before = time.Now()
	}
	// TODO(bradfitz): handle before efficiently. don't use queryPrefix.
	it := x.queryPrefix(keyRecentPermanode, keyId)
	defer closeIterator(it, &err)
	for it.Next() {
		permaStr := it.Value()
		parts := strings.SplitN(it.Key(), "|", 4)
		if len(parts) != 4 {
			continue
		}
		mTime, _ := time.Parse(time.RFC3339, unreverseTimeString(parts[2]))
		permaRef, ok := blob.Parse(permaStr)
		if !ok {
			continue
		}
		if x.IsDeleted(permaRef) {
			continue
		}
		if seenPermanode.Dup(permaStr) {
			continue
		}
		// Skip entries with an mTime less than or equal to before.
		if !mTime.Before(before) {
			continue
		}
		dest <- camtypes.RecentPermanode{
			Permanode:   permaRef,
			Signer:      owner, // TODO(bradfitz): kinda. usually. for now.
			LastModTime: mTime,
		}
		sent++
		if sent == limit {
			break
		}
	}
	return nil
}

func (x *Index) AppendClaims(dst []camtypes.Claim, permaNode blob.Ref,
	signerFilter blob.Ref,
	attrFilter string) ([]camtypes.Claim, error) {
	if x.corpus != nil {
		return x.corpus.AppendClaims(dst, permaNode, signerFilter, attrFilter)
	}
	var (
		keyId string
		err   error
		it    sorted.Iterator
	)
	if signerFilter.Valid() {
		keyId, err = x.KeyId(signerFilter)
		if err == sorted.ErrNotFound {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		it = x.queryPrefix(keyPermanodeClaim, permaNode, keyId)
	} else {
		it = x.queryPrefix(keyPermanodeClaim, permaNode)
	}
	defer closeIterator(it, &err)

	// In the common case, an attribute filter is just a plain
	// token ("camliContent") unescaped. If so, fast path that
	// check to skip the row before we even split it.
	var mustHave string
	if attrFilter != "" && urle(attrFilter) == attrFilter {
		mustHave = attrFilter
	}

	for it.Next() {
		val := it.Value()
		if mustHave != "" && !strings.Contains(val, mustHave) {
			continue
		}
		cl, ok := kvClaim(it.Key(), val, blob.Parse)
		if !ok {
			continue
		}
		if x.IsDeleted(cl.BlobRef) {
			continue
		}
		if attrFilter != "" && cl.Attr != attrFilter {
			continue
		}
		if signerFilter.Valid() && cl.Signer != signerFilter {
			continue
		}
		dst = append(dst, cl)
	}
	return dst, nil
}

func kvClaim(k, v string, blobParse func(string) (blob.Ref, bool)) (c camtypes.Claim, ok bool) {
	const nKeyPart = 5
	const nValPart = 4
	var keya [nKeyPart]string
	var vala [nValPart]string
	keyPart := strutil.AppendSplitN(keya[:0], k, "|", -1)
	valPart := strutil.AppendSplitN(vala[:0], v, "|", -1)
	if len(keyPart) < nKeyPart || len(valPart) < nValPart {
		return
	}
	signerRef, ok := blobParse(valPart[3])
	if !ok {
		return
	}
	permaNode, ok := blobParse(keyPart[1])
	if !ok {
		return
	}
	claimRef, ok := blobParse(keyPart[4])
	if !ok {
		return
	}
	date, err := time.Parse(time.RFC3339, keyPart[3])
	if err != nil {
		return
	}
	return camtypes.Claim{
		BlobRef:   claimRef,
		Signer:    signerRef,
		Permanode: permaNode,
		Date:      date,
		Type:      urld(valPart[0]),
		Attr:      urld(valPart[1]),
		Value:     urld(valPart[2]),
	}, true
}

func (x *Index) GetBlobMeta(br blob.Ref) (camtypes.BlobMeta, error) {
	if x.corpus != nil {
		return x.corpus.GetBlobMeta(br)
	}
	key := "meta:" + br.String()
	meta, err := x.s.Get(key)
	if err == sorted.ErrNotFound {
		err = os.ErrNotExist
	}
	if err != nil {
		return camtypes.BlobMeta{}, err
	}
	pos := strings.Index(meta, "|")
	if pos < 0 {
		panic(fmt.Sprintf("Bogus index row for key %q: got value %q", key, meta))
	}
	size, err := strconv.ParseUint(meta[:pos], 10, 32)
	if err != nil {
		return camtypes.BlobMeta{}, err
	}
	mime := meta[pos+1:]
	return camtypes.BlobMeta{
		Ref:       br,
		Size:      uint32(size),
		CamliType: camliTypeFromMIME(mime),
	}, nil
}

func (x *Index) KeyId(signer blob.Ref) (string, error) {
	if x.corpus != nil {
		return x.corpus.KeyId(signer)
	}
	return x.s.Get("signerkeyid:" + signer.String())
}

func (x *Index) PermanodeOfSignerAttrValue(signer blob.Ref, attr, val string) (permaNode blob.Ref, err error) {
	keyId, err := x.KeyId(signer)
	if err == sorted.ErrNotFound {
		return blob.Ref{}, os.ErrNotExist
	}
	if err != nil {
		return blob.Ref{}, err
	}
	it := x.queryPrefix(keySignerAttrValue, keyId, attr, val)
	defer closeIterator(it, &err)
	for it.Next() {
		permaRef, ok := blob.Parse(it.Value())
		if ok && !x.IsDeleted(permaRef) {
			return permaRef, nil
		}
	}
	return blob.Ref{}, os.ErrNotExist
}

// This is just like PermanodeOfSignerAttrValue except we return multiple and dup-suppress.
// If request.Query is "", it is not used in the prefix search.
func (x *Index) SearchPermanodesWithAttr(dest chan<- blob.Ref, request *camtypes.PermanodeByAttrRequest) (err error) {
	defer close(dest)
	if request.FuzzyMatch {
		// TODO(bradfitz): remove this for now? figure out how to handle it generically?
		return errors.New("TODO: SearchPermanodesWithAttr: generic indexer doesn't support FuzzyMatch on PermanodeByAttrRequest")
	}
	if request.Attribute == "" {
		return errors.New("index: missing Attribute in SearchPermanodesWithAttr")
	}

	keyId, err := x.KeyId(request.Signer)
	if err == sorted.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	var it sorted.Iterator
	if request.Query == "" {
		it = x.queryPrefix(keySignerAttrValue, keyId, request.Attribute)
	} else {
		it = x.queryPrefix(keySignerAttrValue, keyId, request.Attribute, request.Query)
	}
	defer closeIterator(it, &err)
	for it.Next() {
		cl, ok := kvSignerAttrValue(it.Key(), it.Value())
		if !ok {
			continue
		}
		if x.IsDeleted(cl.BlobRef) {
			continue
		}
		if x.IsDeleted(cl.Permanode) {
			continue
		}
		pnstr := cl.Permanode.String()
		if seen[pnstr] {
			continue
		}
		seen[pnstr] = true

		dest <- cl.Permanode
		if len(seen) == request.MaxResults {
			break
		}
	}
	return nil
}

func kvSignerAttrValue(k, v string) (c camtypes.Claim, ok bool) {
	// TODO(bradfitz): garbage
	keyPart := strings.Split(k, "|")
	valPart := strings.Split(v, "|")
	if len(keyPart) != 6 || len(valPart) != 1 {
		// TODO(mpl): use glog
		log.Printf("bogus keySignerAttrValue index entry: %q = %q", k, v)
		return
	}
	if keyPart[0] != "signerattrvalue" {
		return
	}
	date, err := time.Parse(time.RFC3339, unreverseTimeString(keyPart[4]))
	if err != nil {
		log.Printf("bogus time in keySignerAttrValue index entry: %q", keyPart[4])
		return
	}
	claimRef, ok := blob.Parse(keyPart[5])
	if !ok {
		log.Printf("bogus claim in keySignerAttrValue index entry: %q", keyPart[5])
		return
	}
	permaNode, ok := blob.Parse(valPart[0])
	if !ok {
		log.Printf("bogus permanode in keySignerAttrValue index entry: %q", valPart[0])
		return
	}
	return camtypes.Claim{
		BlobRef:   claimRef,
		Permanode: permaNode,
		Date:      date,
		Attr:      urld(keyPart[2]),
		Value:     urld(keyPart[3]),
	}, true
}

func (x *Index) PathsOfSignerTarget(signer, target blob.Ref) (paths []*camtypes.Path, err error) {
	paths = []*camtypes.Path{}
	keyId, err := x.KeyId(signer)
	if err != nil {
		if err == sorted.ErrNotFound {
			err = nil
		}
		return
	}

	mostRecent := make(map[string]*camtypes.Path)
	maxClaimDates := make(map[string]time.Time)

	it := x.queryPrefix(keyPathBackward, keyId, target)
	defer closeIterator(it, &err)
	for it.Next() {
		p, ok, active := kvPathBackward(it.Key(), it.Value())
		if !ok {
			continue
		}
		if x.IsDeleted(p.Claim) {
			continue
		}
		if x.IsDeleted(p.Base) {
			continue
		}

		key := p.Base.String() + "/" + p.Suffix
		if p.ClaimDate.After(maxClaimDates[key]) {
			maxClaimDates[key] = p.ClaimDate
			if active {
				mostRecent[key] = &p
			} else {
				delete(mostRecent, key)
			}
		}
	}
	for _, v := range mostRecent {
		paths = append(paths, v)
	}
	return paths, nil
}

func kvPathBackward(k, v string) (p camtypes.Path, ok bool, active bool) {
	// TODO(bradfitz): garbage
	keyPart := strings.Split(k, "|")
	valPart := strings.Split(v, "|")
	if len(keyPart) != 4 || len(valPart) != 4 {
		// TODO(mpl): use glog
		log.Printf("bogus keyPathBackward index entry: %q = %q", k, v)
		return
	}
	if keyPart[0] != "signertargetpath" {
		return
	}
	target, ok := blob.Parse(keyPart[2])
	if !ok {
		log.Printf("bogus target in keyPathBackward index entry: %q", keyPart[2])
		return
	}
	claim, ok := blob.Parse(keyPart[3])
	if !ok {
		log.Printf("bogus claim in keyPathBackward index entry: %q", keyPart[3])
		return
	}
	date, err := time.Parse(time.RFC3339, valPart[0])
	if err != nil {
		log.Printf("bogus date in keyPathBackward index entry: %q", valPart[0])
		return
	}
	base, ok := blob.Parse(valPart[1])
	if !ok {
		log.Printf("bogus base in keyPathBackward index entry: %q", valPart[1])
		return
	}
	if valPart[2] == "Y" {
		active = true
	}
	return camtypes.Path{
		Claim:     claim,
		Base:      base,
		Target:    target,
		ClaimDate: date,
		Suffix:    urld(valPart[3]),
	}, true, active
}

func (x *Index) PathsLookup(signer, base blob.Ref, suffix string) (paths []*camtypes.Path, err error) {
	paths = []*camtypes.Path{}
	keyId, err := x.KeyId(signer)
	if err != nil {
		if err == sorted.ErrNotFound {
			err = nil
		}
		return
	}

	it := x.queryPrefix(keyPathForward, keyId, base, suffix)
	defer closeIterator(it, &err)
	for it.Next() {
		p, ok, active := kvPathForward(it.Key(), it.Value())
		if !ok {
			continue
		}
		if x.IsDeleted(p.Claim) {
			continue
		}
		if x.IsDeleted(p.Target) {
			continue
		}

		// TODO(bradfitz): investigate what's up with deleted
		// forward path claims here.  Needs docs with the
		// interface too, and tests.
		_ = active

		paths = append(paths, &p)
	}
	return
}

func kvPathForward(k, v string) (p camtypes.Path, ok bool, active bool) {
	// TODO(bradfitz): garbage
	keyPart := strings.Split(k, "|")
	valPart := strings.Split(v, "|")
	if len(keyPart) != 6 || len(valPart) != 2 {
		// TODO(mpl): use glog
		log.Printf("bogus keyPathForward index entry: %q = %q", k, v)
		return
	}
	if keyPart[0] != "path" {
		return
	}
	base, ok := blob.Parse(keyPart[2])
	if !ok {
		log.Printf("bogus base in keyPathForward index entry: %q", keyPart[2])
		return
	}
	date, err := time.Parse(time.RFC3339, unreverseTimeString(keyPart[4]))
	if err != nil {
		log.Printf("bogus date in keyPathForward index entry: %q", keyPart[4])
		return
	}
	claim, ok := blob.Parse(keyPart[5])
	if !ok {
		log.Printf("bogus claim in keyPathForward index entry: %q", keyPart[5])
		return
	}
	if valPart[0] == "Y" {
		active = true
	}
	target, ok := blob.Parse(valPart[1])
	if !ok {
		log.Printf("bogus target in keyPathForward index entry: %q", valPart[1])
		return
	}
	return camtypes.Path{
		Claim:     claim,
		Base:      base,
		Target:    target,
		ClaimDate: date,
		Suffix:    urld(keyPart[3]),
	}, true, active
}

func (x *Index) PathLookup(signer, base blob.Ref, suffix string, at time.Time) (*camtypes.Path, error) {
	paths, err := x.PathsLookup(signer, base, suffix)
	if err != nil {
		return nil, err
	}
	var (
		newest    = int64(0)
		atSeconds = int64(0)
		best      *camtypes.Path
	)

	if !at.IsZero() {
		atSeconds = at.Unix()
	}

	for _, path := range paths {
		t := path.ClaimDate
		secs := t.Unix()
		if atSeconds != 0 && secs > atSeconds {
			// Too new
			continue
		}
		if newest > secs {
			// Too old
			continue
		}
		// Just right
		newest, best = secs, path
	}
	if best == nil {
		return nil, os.ErrNotExist
	}
	return best, nil
}

func (x *Index) ExistingFileSchemas(wholeRef blob.Ref) (schemaRefs []blob.Ref, err error) {
	it := x.queryPrefix(keyWholeToFileRef, wholeRef)
	defer closeIterator(it, &err)
	for it.Next() {
		keyPart := strings.Split(it.Key(), "|")[1:]
		if len(keyPart) < 2 {
			continue
		}
		ref, ok := blob.Parse(keyPart[1])
		if ok {
			schemaRefs = append(schemaRefs, ref)
		}
	}
	return schemaRefs, nil
}

func (x *Index) loadKey(key string, val *string, err *error, wg *sync.WaitGroup) {
	defer wg.Done()
	*val, *err = x.s.Get(key)
}

func (x *Index) GetFileInfo(fileRef blob.Ref) (camtypes.FileInfo, error) {
	if x.corpus != nil {
		return x.corpus.GetFileInfo(fileRef)
	}
	ikey := "fileinfo|" + fileRef.String()
	tkey := "filetimes|" + fileRef.String()
	wg := new(sync.WaitGroup)
	wg.Add(2)
	var iv, tv string // info value, time value
	var ierr, terr error
	go x.loadKey(ikey, &iv, &ierr, wg)
	go x.loadKey(tkey, &tv, &terr, wg)
	wg.Wait()

	if ierr == sorted.ErrNotFound {
		return camtypes.FileInfo{}, os.ErrNotExist
	}
	if ierr != nil {
		return camtypes.FileInfo{}, ierr
	}
	valPart := strings.Split(iv, "|")
	if len(valPart) < 3 {
		log.Printf("index: bogus key %q = %q", ikey, iv)
		return camtypes.FileInfo{}, os.ErrNotExist
	}
	size, err := strconv.ParseInt(valPart[0], 10, 64)
	if err != nil {
		log.Printf("index: bogus integer at position 0 in key %q = %q", ikey, iv)
		return camtypes.FileInfo{}, os.ErrNotExist
	}
	fileName := urld(valPart[1])
	fi := camtypes.FileInfo{
		Size:     size,
		FileName: fileName,
		MIMEType: urld(valPart[2]),
	}

	if tv != "" {
		times := strings.Split(urld(tv), ",")
		updateFileInfoTimes(&fi, times)
	}

	return fi, nil
}

func updateFileInfoTimes(fi *camtypes.FileInfo, times []string) {
	if len(times) == 0 {
		return
	}
	fi.Time = types.ParseTime3339OrNil(times[0])
	if len(times) == 2 {
		fi.ModTime = types.ParseTime3339OrNil(times[1])
	}
}

// v is "width|height"
func kvImageInfo(v []byte) (ii camtypes.ImageInfo, ok bool) {
	pipei := bytes.IndexByte(v, '|')
	if pipei < 0 {
		return
	}
	w, err := strutil.ParseUintBytes(v[:pipei], 10, 16)
	if err != nil {
		return
	}
	h, err := strutil.ParseUintBytes(v[pipei+1:], 10, 16)
	if err != nil {
		return
	}
	ii.Width = uint16(w)
	ii.Height = uint16(h)
	return ii, true
}

func (x *Index) GetImageInfo(fileRef blob.Ref) (camtypes.ImageInfo, error) {
	if x.corpus != nil {
		return x.corpus.GetImageInfo(fileRef)
	}
	// it might be that the key does not exist because image.DecodeConfig failed earlier
	// (because of unsupported JPEG features like progressive mode).
	key := keyImageSize.Key(fileRef.String())
	v, err := x.s.Get(key)
	if err == sorted.ErrNotFound {
		err = os.ErrNotExist
	}
	if err != nil {
		return camtypes.ImageInfo{}, err
	}
	ii, ok := kvImageInfo([]byte(v))
	if !ok {
		return camtypes.ImageInfo{}, fmt.Errorf("index: bogus key %q = %q", key, v)
	}
	return ii, nil
}

func (x *Index) EdgesTo(ref blob.Ref, opts *camtypes.EdgesToOpts) (edges []*camtypes.Edge, err error) {
	it := x.queryPrefix(keyEdgeBackward, ref)
	defer closeIterator(it, &err)
	permanodeParents := make(map[string]*camtypes.Edge)
	for it.Next() {
		edge, ok := kvEdgeBackward(it.Key(), it.Value())
		if !ok {
			continue
		}
		if x.IsDeleted(edge.From) {
			continue
		}
		if x.IsDeleted(edge.BlobRef) {
			continue
		}
		edge.To = ref
		if edge.FromType == "permanode" {
			permanodeParents[edge.From.String()] = edge
		} else {
			edges = append(edges, edge)
		}
	}
	for _, e := range permanodeParents {
		edges = append(edges, e)
	}
	return edges, nil
}

func kvEdgeBackward(k, v string) (edge *camtypes.Edge, ok bool) {
	// TODO(bradfitz): garbage
	keyPart := strings.Split(k, "|")
	valPart := strings.Split(v, "|")
	if len(keyPart) != 4 || len(valPart) != 2 {
		// TODO(mpl): use glog
		log.Printf("bogus keyEdgeBackward index entry: %q = %q", k, v)
		return
	}
	if keyPart[0] != "edgeback" {
		return
	}
	parentRef, ok := blob.Parse(keyPart[2])
	if !ok {
		log.Printf("bogus parent in keyEdgeBackward index entry: %q", keyPart[2])
		return
	}
	blobRef, ok := blob.Parse(keyPart[3])
	if !ok {
		log.Printf("bogus blobref in keyEdgeBackward index entry: %q", keyPart[3])
		return
	}
	return &camtypes.Edge{
		From:      parentRef,
		FromType:  valPart[0],
		FromTitle: valPart[1],
		BlobRef:   blobRef,
	}, true
}

// GetDirMembers sends on dest the children of the static directory dir.
func (x *Index) GetDirMembers(dir blob.Ref, dest chan<- blob.Ref, limit int) (err error) {
	defer close(dest)

	sent := 0
	it := x.queryPrefix(keyStaticDirChild, dir.String())
	defer closeIterator(it, &err)
	for it.Next() {
		keyPart := strings.Split(it.Key(), "|")
		if len(keyPart) != 3 {
			return fmt.Errorf("index: bogus key keyStaticDirChild = %q", it.Key())
		}

		child, ok := blob.Parse(keyPart[2])
		if !ok {
			continue
		}
		dest <- child
		sent++
		if sent == limit {
			break
		}
	}
	return nil
}

func kvBlobMeta(k, v string) (bm camtypes.BlobMeta, ok bool) {
	refStr := k[len("meta:"):]
	br, ok := blob.Parse(refStr)
	if !ok {
		return
	}
	pipe := strings.Index(v, "|")
	if pipe < 0 {
		return
	}
	size, err := strconv.ParseUint(v[:pipe], 10, 32)
	if err != nil {
		return
	}
	return camtypes.BlobMeta{
		Ref:       br,
		Size:      uint32(size),
		CamliType: camliTypeFromMIME(v[pipe+1:]),
	}, true
}

func kvBlobMeta_bytes(k, v []byte) (bm camtypes.BlobMeta, ok bool) {
	ref := k[len("meta:"):]
	br, ok := blob.ParseBytes(ref)
	if !ok {
		return
	}
	pipe := bytes.IndexByte(v, '|')
	if pipe < 0 {
		return
	}
	size, err := strutil.ParseUintBytes(v[:pipe], 10, 32)
	if err != nil {
		return
	}
	return camtypes.BlobMeta{
		Ref:       br,
		Size:      uint32(size),
		CamliType: camliTypeFromMIME_bytes(v[pipe+1:]),
	}, true
}

func enumerateBlobMeta(s sorted.KeyValue, cb func(camtypes.BlobMeta) error) (err error) {
	it := queryPrefixString(s, "meta:")
	defer closeIterator(it, &err)
	for it.Next() {
		bm, ok := kvBlobMeta(it.Key(), it.Value())
		if !ok {
			continue
		}
		if err := cb(bm); err != nil {
			return err
		}
	}
	return nil
}

func enumerateSignerKeyId(s sorted.KeyValue, cb func(blob.Ref, string)) (err error) {
	const pfx = "signerkeyid:"
	it := queryPrefixString(s, pfx)
	defer closeIterator(it, &err)
	for it.Next() {
		if br, ok := blob.Parse(strings.TrimPrefix(it.Key(), pfx)); ok {
			cb(br, it.Value())
		}
	}
	return
}

// EnumerateBlobMeta sends all metadata about all known blobs to ch and then closes ch.
func (x *Index) EnumerateBlobMeta(ctx *context.Context, ch chan<- camtypes.BlobMeta) (err error) {
	if x.corpus != nil {
		x.corpus.RLock()
		defer x.corpus.RUnlock()
		return x.corpus.EnumerateBlobMetaLocked(ctx, ch)
	}
	defer close(ch)
	return enumerateBlobMeta(x.s, func(bm camtypes.BlobMeta) error {
		select {
		case ch <- bm:
		case <-ctx.Done():
			return context.ErrCanceled
		}
		return nil
	})
}

// Storage returns the index's underlying Storage implementation.
func (x *Index) Storage() sorted.KeyValue { return x.s }

// Close closes the underlying sorted.KeyValue, if the storage has a Close method.
// The return value is the return value of the underlying Close, or
// nil otherwise.
func (x *Index) Close() error {
	if cl, ok := x.s.(io.Closer); ok {
		return cl.Close()
	}
	return nil
}

const camliTypeMIMEPrefix = "application/json; camliType="

var camliTypeMIMEPrefixBytes = []byte(camliTypeMIMEPrefix)

// "application/json; camliType=file" => "file"
// "image/gif" => ""
func camliTypeFromMIME(mime string) string {
	if v := strings.TrimPrefix(mime, camliTypeMIMEPrefix); v != mime {
		return v
	}
	return ""
}

func camliTypeFromMIME_bytes(mime []byte) string {
	if v := bytes.TrimPrefix(mime, camliTypeMIMEPrefixBytes); len(v) != len(mime) {
		return strutil.StringFromBytes(v)
	}
	return ""
}

// TODO(bradfitz): rename this? This is really about signer-attr-value
// (PermanodeOfSignerAttrValue), and not about indexed attributes in general.
func IsIndexedAttribute(attr string) bool {
	switch attr {
	case "camliRoot", "camliImportRoot", "tag", "title":
		return true
	}
	return false
}

// IsBlobReferenceAttribute returns whether attr is an attribute whose
// value is a blob reference (e.g. camliMember) and thus something the
// indexers should keep inverted indexes on for parent/child-type
// relationships.
func IsBlobReferenceAttribute(attr string) bool {
	switch attr {
	case "camliMember":
		return true
	}
	return false
}

func IsFulltextAttribute(attr string) bool {
	switch attr {
	case "tag", "title":
		return true
	}
	return false
}
