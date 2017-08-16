package varroa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebHooks(t *testing.T) {
	fmt.Println("+ Testing Webhooks...")
	check := assert.New(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		check.True(r.Method == "POST")
		eventType := r.Header.Get("X-Varroa-Event")
		check.Equal("info", eventType)
		token := r.Header.Get("X-Varroa-Token")
		check.Equal("token", token)

		body, err := ioutil.ReadAll(r.Body)
		check.Nil(err)

		var wbJSON WebHookJSON
		check.Nil(json.Unmarshal(body, &wbJSON))
		check.Equal("site", wbJSON.Site)
		check.Equal("msg", wbJSON.Message)
		check.Equal("info", wbJSON.Type)
		check.Equal("http://link.link", wbJSON.Link)
	}))
	defer ts.Close()

	wh := &WebHookJSON{Site: "site", Message: "msg", Type: "info", Link: "http://link.link"}
	err := wh.Send(ts.URL, "token")
	check.Nil(err)
}
