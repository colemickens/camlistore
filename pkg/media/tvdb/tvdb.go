package tvdb

// cache locally
// check for periodic updates, etc
//

import (
	"encoding/xml"
	"io"
	"net/url"
	"strings"
	//"log"
	"archive/zip"
	"bytes"
	"fmt"
	"log"
	"net/http"
)

const (
	mirror = "http://thetvdb.com/" // their wiki says this can be hard-coded
)

type TvdbApi struct {
	ApiKey   string
	Client   *http.Client
	Language string
}

var cachedSeriesResps = make(map[string]*SeriesListResponse)
var cachedLangResps = make(map[int]*LangResponse)

func NewTvdbApi(apiKey string, client *http.Client) (*TvdbApi, error) {
	if client == nil {
		client = &http.Client{}
	}
	// should probably fire a test query and return err if we dont get an expected result
	return &TvdbApi{apiKey, client, "en"}, nil
}

func fetch(url *url.URL, obj interface{}) error {
	log.Println(url.String())
	resp, err := http.Get(url.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	xmlDec := xml.NewDecoder(resp.Body)

	if err = xmlDec.Decode(obj); err != nil {
		return err
	}

	return nil
}

func (t *TvdbApi) Show(seriesId int) {

}

// TODO: make this thread safe
func (t *TvdbApi) GetSeriesData(seriesId int) (ser *LangResponse, e error) {
	if cachedLangResp, ok := cachedLangResps[seriesId]; ok {
		return cachedLangResp, nil
	}

	url := mirror + "/api/" + t.ApiKey + "/series/" + fmt.Sprintf("%d", seriesId) + "/all/" + t.Language + ".zip"

	log.Println("GET " + url)

	buf := &bytes.Buffer{}
	resp, err := http.Get(url)
	io.Copy(buf, resp.Body)
	resp.Body.Close()

	reader := bytes.NewReader(buf.Bytes())
	r, err := zip.NewReader(reader, int64(reader.Len()))
	if err != nil {
		return nil, fmt.Errorf("Failed to open zip reader for series id: " + fmt.Sprintf("%d", seriesId))
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			log.Println("error reading file in ExtendedInfo TVDB")
		}

		switch f.Name {
		case "banners.xml":
			/*
				xmlDec := xml.NewDecoder(rc)
				br = &BannersResponse{}
				err := xmlDec.Decode(br)
				if err != nil {
					log.Println("failed to decode banners.xml in ExtendedInfo TVDB")
				}
			*/
		case "actors.xml":
			/*
				xmlDec := xml.NewDecoder(rc)
				ar = &ActorsResponse{}
				err := xmlDec.Decode(ar)
				if err != nil {
					log.Println("failed to decode actors.xml in ExtendedInfo TVDB")
				}
			*/
		case t.Language + ".xml":
			xmlDec := xml.NewDecoder(rc)
			ser = &LangResponse{}
			err := xmlDec.Decode(ser)
			if err != nil {
				panic(err)
			}
		default:
			//log.Println("unknown file in ExtendedInfo TVDB")
		}
	}

	// put it in the cachedExtendedInfo
	cachedLangResps[seriesId] = ser
	if ser == nil {
		return nil, fmt.Errorf(t.Language+".xml was missing for series id: %d", seriesId)
	}

	return
}

func (t *TvdbApi) SearchSeriesByName(seriesName string) ([]*Series, error) {
	seriesName = strings.ToLower(seriesName)

	if cachedSeriesResp, ok := cachedSeriesResps[seriesName]; ok {
		return cachedSeriesResp.Series, nil
	}
	values := &url.Values{"seriesname": []string{seriesName}, "language": []string{t.Language}}
	url := getUrl("api/GetSeries.php", values)
	log.Println(url)
	url.Query().Add("seriesname", seriesName)
	url.Query().Add("language", t.Language)
	log.Println(url)

	slr := &SeriesListResponse{}
	if err := fetch(url, slr); err != nil {
		return nil, err
	}

	cachedSeriesResps[seriesName] = slr

	return slr.Series, nil
}

func getUrl(relativePath string, values *url.Values) *url.URL {
	copyurl, err := url.Parse(mirror + relativePath)
	if err != nil {
		panic(err)
	}
	copyurl.RawQuery = values.Encode()
	return copyurl
}
