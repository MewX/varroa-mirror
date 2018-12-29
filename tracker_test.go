package varroa

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type HTTPBinResponse struct {
	Args    struct{} `json:"args"`
	Headers struct {
		AcceptEncoding string `json:"Accept-Encoding"`
		Connection     string `json:"Connection"`
		Host           string `json:"Host"`
		UserAgent      string `json:"User-Agent"`
	} `json:"headers"`
	Origin string `json:"origin"`
	URL    string `json:"url"`
}

func TestTrackerAPI(t *testing.T) {
	fmt.Println("+ Testing Tracker...")

	// setting up
	verify := assert.New(t)
	env := NewEnvironment()
	logThis = NewLogThis(env)

	tracker := &GazelleTracker{Name: "test TRKR", URL: "http://httpbin.org", User: "user", Password: "password", limiter: make(chan bool, allowedAPICallsByPeriod)}
	tracker.client = &http.Client{}

	go tracker.apiCallRateLimiter()

	// get time
	start := time.Now()

	for i := 0; i < 11; i++ {
		fmt.Printf("Waiting to send GET #%d\n", i+1)
		data, err := tracker.rateLimitedGetRequest(tracker.URL+"/get", false)
		fmt.Printf("%s: Sent GET #%d\n", time.Since(start).String(), i+1)
		verify.Nil(err)
		var r HTTPBinResponse
		verify.Nil(json.Unmarshal(data, &r))
		verify.Equal(userAgent(), r.Headers.UserAgent)
	}

	elapsed := time.Since(start)
	fmt.Printf("11 GET calls in %s\n", elapsed.String())
	verify.True(elapsed.Seconds() > 20)
	fmt.Println("If the 11th call is sent after 20s, it means 10 calls were sent in 20s, i.e. we have successfully rate-limited to 5reqs/10s")
}
