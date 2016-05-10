package mobileproxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/html"
)

func clientCanGzip(req *http.Request) bool {
	return strings.Contains(req.Header.Get("Accept-encoding"), "gzip")
}

func responseAlreadyEncoded(resp *http.Response) bool {
	ce := resp.Header.Get("Content-encoding")
	if ce != "" {
		return true
	}
	return len(resp.TransferEncoding) > 0
}

var compressedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"video/mp4":  true,
}

func isWorthGzipping(resp *http.Response) bool {
	if resp.ContentLength < 1024 {
		return false
	}

	if resp.Header.Get("Content-type") == "" {
		// putting content-type: gzip on something without a content-type doesn't work right
		return false
	}

	alreadyCompressed := compressedMimeTypes[resp.Header.Get("Content-type")]
	return !alreadyCompressed
}

func do(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}
	u, err := url.Parse(r.RequestURI)
	if err != nil {
		http.Error(w, "Bad URL", 400)
		return
	}
	newReq := http.Request{
		Method: r.Method,
		URL:    u,
		Header: r.Header,
		Close:  r.Close,
		Body:   r.Body,
	}
	c := http.Client{}
	srvResp, err := c.Do(&newReq)
	if err != nil {
		http.Error(w, "Proxy failed with backend request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer srvResp.Body.Close()
	for k, vs := range srvResp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Add("X-Rob-Proxy", "0.1")

	// if clientCanGzip(r) && !responseAlreadyEncoded(srvResp) && isWorthGzipping(srvResp) {
	// 	// gzip the content
	// 	// seeing we don't know the output gzip length, we have to chunk it and throw away any existing content-length header
	// 	w.Header().Set("Content-encoding", "gzip")
	// 	w.Header().Set("Transfer-encoding", "chunked")
	// 	w.Header().Del("Content-length")
	// 	w.WriteHeader(srvResp.StatusCode)
	// 	cw := httputil.NewChunkedWriter(w)
	// 	gw := gzip.NewWriter(cw)
	// 	io.Copy(gw, srvResp.Body)
	// 	gw.Close()
	// 	cw.Close()
	// } else {
	// 	w.WriteHeader(srvResp.StatusCode)
	// 	io.Copy(w, srvResp.Body)
	// }

	// copy server response headers to client response headers
	for k, vs := range srvResp.Header {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}

	// special handling of HTML :D
	if strings.HasPrefix(srvResp.Header.Get("Content-type"), "text/html") {
		w.Header().Del("Content-length")
		w.Header().Del("Content-encoding")
		w.Header().Add("Transfer-encoding", "chunked")
		// server body could be gzipped
		reader, err := getRawReader(srvResp)
		if err != nil {
			http.Error(w, "Failed to get raw server response body: "+err.Error(), http.StatusBadGateway)
			return
		}

		w.WriteHeader(srvResp.StatusCode)
		w.Header().Write(os.Stdout)

		if err = processAndPassThroughHTML(reader, w); err != nil {
			log.Printf("Failed to parse %s - %s", r.URL.String(), err.Error())
		}
	} else {
		w.WriteHeader(srvResp.StatusCode)
		io.Copy(w, srvResp.Body)
	}
}

func getRawReader(resp *http.Response) (io.Reader, error) {
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-encoding") == "gzip" {
		var err error
		log.Print("Applying a gzip reader")
		reader, err = gzip.NewReader(reader)
		if err != nil {
			log.Print("Oh dears: " + err.Error())
			return nil, err
		}
	}
	return reader, nil
}

func unknownMethodHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Not sure how to do a %s request for %s", r.Method, r.URL.String())

	http.Error(w, "Unknown request method: "+r.Method, http.StatusBadRequest)
}

func processAndPassThroughHTML(r io.Reader, w io.Writer) error {
	z := html.NewTokenizer(r)
	indent := 0
	script := false
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		case html.StartTagToken:
			name, hasAttributes := z.TagName()
			writeIndent(w, indent)
			w.Write([]byte("<"))
			w.Write(name)
			if hasAttributes {
				moreAttr := true
				var key []byte
				var val []byte
				for moreAttr {
					w.Write([]byte(" "))
					key, val, moreAttr = z.TagAttr()
					w.Write(key)
					if len(val) > 0 {
						w.Write([]byte(`="`))
						w.Write([]byte(escapeAttributeValue(string(val))))
						w.Write([]byte(`"`))
					}
				}
			}
			w.Write([]byte(">\n"))
			indent++
			script = bytes.Equal([]byte("script"), name)
		case html.EndTagToken:
			w.Write([]byte("</"))
			tagName, _ := z.TagName()
			w.Write(tagName)
			w.Write([]byte(">\n"))
			indent--
			script = false
		case html.SelfClosingTagToken:
			name, hasAttributes := z.TagName()
			writeIndent(w, indent)
			w.Write([]byte("<"))
			w.Write(name)
			if hasAttributes {
				moreAttr := true
				var key []byte
				var val []byte
				for moreAttr {
					w.Write([]byte(" "))
					key, val, moreAttr = z.TagAttr()
					w.Write(key)
					if len(val) > 0 {
						w.Write([]byte(`="`))
						w.Write([]byte(escapeAttributeValue(string(val))))
						w.Write([]byte(`"`))
					}
				}
			}
			w.Write([]byte("/>\n"))
		case html.TextToken:
			if script {
				w.Write(z.Text())
			} else {
				str := strings.TrimSpace(string(z.Text()))
				if str != "" {
					writeIndent(w, indent)
					w.Write([]byte(html.EscapeString(str)))
					w.Write([]byte("\n"))
				}
			}
		case html.DoctypeToken:
			w.Write([]byte("<!"))
			w.Write(z.Text())
			w.Write([]byte(">\n"))
		}
	}
}

func escapeAttributeValue(s string) string {
	return strings.Replace(strings.Replace(s, `\`, `\\`, -1), `"`, `\"`, -1)
}

func writeIndent(w io.Writer, indent int) {
	b := make([]byte, indent)
	for i := 0; i < indent; i++ {
		b[i] = ' '
	}
	w.Write(b)
}
