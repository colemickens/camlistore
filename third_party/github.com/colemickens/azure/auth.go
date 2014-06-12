package azure

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type Auth struct {
	Account string
	Key     string
}

func (a *Auth) SignRequest(req *http.Request) {
	strToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s",
		strings.ToUpper(req.Method),
		tryget(req.Header, "Content-Encoding"),
		tryget(req.Header, "Content-Language"),
		tryget(req.Header, "Content-Length"),
		tryget(req.Header, "Content-MD5"),
		tryget(req.Header, "Content-Type"),
		tryget(req.Header, "Date"),
		tryget(req.Header, "If-Modified-Since"),
		tryget(req.Header, "If-Match"),
		tryget(req.Header, "If-None-Match"),
		tryget(req.Header, "If-Unmodified-Since"),
		tryget(req.Header, "Range"),
		a.canonicalizedHeaders(req),
		a.canonicalizedResource(req),
	)

	decodedKey, _ := base64.StdEncoding.DecodeString(a.Key)

	sha256 := hmac.New(sha256.New, []byte(decodedKey))
	sha256.Write([]byte(strToSign))

	signature := base64.StdEncoding.EncodeToString(sha256.Sum(nil))

	copyHeadersToRequest(req, map[string]string{
		"Authorization": fmt.Sprintf("SharedKey %s:%s", a.Account, signature),
	})
}

func tryget(headers map[string][]string, key string) string {
	if len(headers[key]) > 0 {
		return headers[key][0]
	}
	return ""
}

//
// The following is copied ~95% verbatim from:
//  http://github.com/loldesign/azure/ -> core/core.go
//

/*
Based on Azure docs:
  Link: http://msdn.microsoft.com/en-us/library/windowsazure/dd179428.aspx#Constructing_Element

  1) Retrieve all headers for the resource that begin with x-ms-, including the x-ms-date header.
  2) Convert each HTTP header name to lowercase.
  3) Sort the headers lexicographically by header name, in ascending order. Note that each header may appear only once in the string.
  4) Unfold the string by replacing any breaking white space with a single space.
  5) Trim any white space around the colon in the header.
  6) Finally, append a new line character to each canonicalized header in the resulting list. Construct the CanonicalizedHeaders string by concatenating all headers in this list into a single string.
*/
func (a *Auth) canonicalizedHeaders(req *http.Request) string {
	var buffer bytes.Buffer

	for key, value := range req.Header {
		lowerKey := strings.ToLower(key)

		if strings.HasPrefix(lowerKey, "x-ms-") {
			if buffer.Len() == 0 {
				buffer.WriteString(fmt.Sprintf("%s:%s", lowerKey, value[0]))
			} else {
				buffer.WriteString(fmt.Sprintf("\n%s:%s", lowerKey, value[0]))
			}
		}
	}

	splitted := strings.Split(buffer.String(), "\n")
	sort.Strings(splitted)

	return strings.Join(splitted, "\n")
}

/*
Based on Azure docs
  Link: http://msdn.microsoft.com/en-us/library/windowsazure/dd179428.aspx#Constructing_Element

1) Beginning with an empty string (""), append a forward slash (/), followed by the name of the account that owns the resource being accessed.
2) Append the resource's encoded URI path, without any query parameters.
3) Retrieve all query parameters on the resource URI, including the comp parameter if it exists.
4) Convert all parameter names to lowercase.
5) Sort the query parameters lexicographically by parameter name, in ascending order.
6) URL-decode each query parameter name and value.
7) Append each query parameter name and value to the string in the following format, making sure to include the colon (:) between the name and the value:
    parameter-name:parameter-value

8) If a query parameter has more than one value, sort all values lexicographically, then include them in a comma-separated list:
    parameter-name:parameter-value-1,parameter-value-2,parameter-value-n

9) Append a new line character (\n) after each name-value pair.

Rules:
  1) Avoid using the new line character (\n) in values for query parameters. If it must be used, ensure that it does not affect the format of the canonicalized resource string.
  2) Avoid using commas in query parameter values.
*/
func (a *Auth) canonicalizedResource(req *http.Request) string {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("/%s%s", a.Account, req.URL.Path))
	queries := req.URL.Query()

	for key, values := range queries {
		sort.Strings(values)
		buffer.WriteString(fmt.Sprintf("\n%s:%s", key, strings.Join(values, ",")))
	}

	splitted := strings.Split(buffer.String(), "\n")
	sort.Strings(splitted)

	return strings.Join(splitted, "\n")
}
