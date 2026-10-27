//go:build localonly

package router

import (
	"bufio"
	"net/http"
	"strings"
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/stretchr/testify/require"
)

// TestReceive runs a captured Mastodon Follow delivery through ReceiveRequest with the default
// validators, and expects it to be accepted.
func TestReceive(t *testing.T) {

	httpReader := strings.NewReader(`POST /@66beb0b36afe0012604c5467/pub/inbox HTTP/1.1
Host: bandwagon.fm
Accept-Encoding: gzip, br
Cdn-Loop: cloudflare; loops=1
Content-Length: 254
Content-Type: application/activity+json
Date: Sat, 28 Sep 2024 13:36:31 GMT
Digest: SHA-256=XMfWai4fSQ5v1fjYoSzTOy4IRa6utDygHVXXYbM0/JM=
Do-Connecting-Ip: 146.255.56.82
Signature: keyId="https://climatejustice.social/users/benpate#main-key",algorithm="rsa-sha256",headers="(request-target) host date digest content-type",signature="R6oGuDCaNIMq0J/j3y5dW5NybHHqSYZAdw4b2pqqhnK/m/uLi2pQKaT1ao6IBwGJukN1CZKXxkUxYZwSozi5bVMg4Z27sCGir1enerYV5tEsz0Oafoa1gxQBlcgHx7lCZhuFNpeqi9CAIyToUayHn3NFhHmvIKFz61PtBAuW64VRWJ6dx/jsFJsytkmzfi+vQCKYGUGMxHIsL1TR0rwUTU5vwfy9PbCNuC1O/3crR8OICdClKoS+1as08Qsx8oEsCSBQb+M1bVsLycQ/6M+hYmu5Qu8wS2pzeOq3LvavwqpXqX6rPQ2kHgToNdtiErZgFsFUgwIL7vRPI5oP0CfpAQ=="
User-Agent: http.rb/5.1.1 (Mastodon/4.2.12-stable+ff1; +https://climatejustice.social/)
X-Forwarded-For: 146.255.56.82,108.162.221.54
X-Forwarded-Proto: https

{"@context":"https://www.w3.org/ns/activitystreams","id":"https://climatejustice.social/1b888e76-d22e-445e-8264-11d2c9bcc46f","type":"Follow","actor":"https://climatejustice.social/users/benpate","object":"https://bandwagon.fm/@66beb0b36afe0012604c5467"}
`)

	client := streams.NewDefaultClient()

	req, err := http.ReadRequest(bufio.NewReader(httpReader))
	require.Nil(t, err)

	document, err := ReceiveRequest(req, client)
	require.Nil(t, err)

	t.Log(document.Value())
}
