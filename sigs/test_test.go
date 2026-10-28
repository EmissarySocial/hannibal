package sigs

import (
	"bytes"
	"net/http"

	"github.com/rs/zerolog"
)

// init sets Unix timestamps and the Warn log level for this package's tests
func init() {
	// Configure logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// must returns value, or panics if err is not nil
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// getTestPixelFedRequest returns a request carrying the body, Digest, and Signature of a captured
// Pixelfed Follow delivery.
func getTestPixelFedRequest() *http.Request {
	var body bytes.Buffer
	body.WriteString(`{"@context":"https:\/\/www.w3.org\/ns\/activitystreams","id":"https:\/\/pixelfed.social\/users\/benpate#follow\/595731146082391369","type":"Follow","actor":"https:\/\/pixelfed.social\/users\/benpate","object":"https:\/\/emdev.ddns.net\/@64d68054a4bf39a519f27c67"}`)

	request, err := http.NewRequest("GET", "https://emdev.ddns.net/@64d68054a4bf39a519f27c67/pub/inbox", &body)

	if err != nil {
		panic(err)
	}

	request.Header.Set("User-Agent", "(Pixelfed/0.11.9; +https://pixelfed.social)")
	request.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	request.Header.Set("Date", "Mon, 04 Sep 2023 21:17:36 GMT")
	request.Header.Set("Digest", "SHA-256=TwwjRc4l0VffR6UXoebZctDg2CY/sxUciFKxzVC3kPo=")
	request.Header.Set("Signature", `keyId="https://pixelfed.social/users/benpate#main-key",headers="(request-target) host date digest content-type user-agent",algorithm="rsa-sha256",signature="ZYIfR4fUvNt7K/2iWxke83wOJCPqnRNhJqPV3Z8NisTeaEXc1ujYAGahTyAUYYY1hKPDJL6HcbPszG5R/7yXUfQVoABDBeWN6k8pVm43FpfCic156qCczvGM6KqzhQtWrw4nYuILzdL+QCJo7O9H6TEsLAuVcJ7ycb5BpiNvOMy9pMnLAyvf8A3qxhh9NYm+PtzFczQ83HBCDtpr7N+wMvP1xhIByouaB0VLntsyjpjJZdQSmZteSiZixN3h27lkBI/++xLdTKbff81dwdMEVROf4HUp/TR5kmh6NotoV7bTGxn/0c05Bv4bpNaMQc8f5myn/r4MuTHwS4pTSlhe/w=="`)

	return request
}
