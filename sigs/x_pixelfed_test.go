package sigs

import (
	"bufio"
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_PixelFed(t *testing.T) {

	raw := removeTabs(`POST /@64d68054a4bf39a519f27c67/pub/inbox HTTP/1.1
	Host: emdev.ddns.net
	Accept: */*
	Date: Sun, 10 Sep 2023 01:12:22 GMT
	Digest: SHA-256=CjcIHEJJriyG8aC9K7mayMOi0drhBb2i4fvGI0t8phk=
	Content-Type: application/ld+json; profile="https://www.w3.org/ns/activitystreams"
	User-Agent: (Pixelfed/0.11.9; +https://pixelfed.social)
	Signature: keyId="https://pixelfed.social/users/benpate#main-key",headers="(request-target) host date digest content-type user-agent",algorithm="rsa-sha256",signature="cxaG7js7JxV5jvnHPqZQhLbIS47BX53A79DGNmVeKUYWQJk5sbsIgC+xvYxc7mZan2yI3EKNyrV/X61hAX1DyexeoGzGSAOmTnC0OdrniaWb3T71Supnej/1In0EuiL0+IXgqOH1AncwnZnYODBYOFOYgtoh2jWlmqKI8uE/L3iKP1nIhN8mUOf//6AqkkwNjz4PmPJ6nS1+gGckszjD1zxjFv2ncgo4rY4izSGVFdAU4QSA8ds3W6qIvha4nRoYeH8ZSzQjaIrM2owa62KgguonbBUa0NGNQHC5RxySj8Kzhw/AmIccKaJw7ythHAH9Km/zDflOVWZ62uurlv7Ikg=="
	Content-Length: 423
	
	{"@context":"https:\/\/www.w3.org\/ns\/activitystreams","id":"https:\/\/pixelfed.social\/users\/benpate#follow\/595731146082391369\/undo","type":"Undo","actor":"https:\/\/pixelfed.social\/users\/benpate","object":{"id":"https:\/\/pixelfed.social\/users\/benpate#follows\/595731146082391369","actor":"https:\/\/pixelfed.social\/users\/benpate","object":"https:\/\/emdev.ddns.net\/@64d68054a4bf39a519f27c67","type":"Follow"}}`)

	keyFinder := func(_ string) (string, error) {
		return removeTabs(
			`-----BEGIN PUBLIC KEY-----
		MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsuI80UzaF1gQinBAZnz7
		CtqH4Rr6Booyii2ik+6Dw6nyuu//1VJVgWkck9xzGP1e0h2pqHNlqvsPxwjxw22o
		J31+hdU9mSOrK3C0k6nBT+RPEgfyj1UCXGk1lzfpKmgriftMGQ2kOQokRLqyKfXJ
		P7tizc3YUAGHw7pBWn/yV6SqMDyztvTF8Bzqx1zG4dvRLDpRWdVOZQkpZ+mrczt7
		TaYiZWkxSjZw0fsnoDXiDHX7cljq67Yrzu0WmeAR7fYAJgCxlC++557w95xY58Z9
		kIbcWXx0AExo/ed1GFNqFwg9Rdx58PzmA8dT9UpBOo9z6lu4KlbuWFYHz7b8HHGs
		rwIDAQAB
		-----END PUBLIC KEY-----`), nil
	}

	requestReader := bufio.NewReader(bytes.NewReader([]byte(raw)))
	request := must(http.ReadRequest(requestReader))

	_, err := Verify(request, keyFinder, VerifierIgnoreTimeout())
	require.Nil(t, err)
}
