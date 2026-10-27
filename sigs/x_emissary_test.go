package sigs

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_Emissary confirms that a captured hs2019 request from Emissary to activitypub.academy verifies
func Test_Emissary(t *testing.T) {

	rawHTTP := removeTabs(
		`POST /users/angusius_dukernen/inbox HTTP/1.1
		Host: activitypub.academy
		Accept: application/activity+json
		Date: Sun, 10 Sep 2023 10:37:22 GMT
		Digest: SHA-256=gH5r8XfomFHwvjZUZAqtbE1MpJkmbgCZ3cQjYGt7ppA=
		Content-Type: application/activity+json
		Signature: keyId="https://emdev.ddns.net/@64d68054a4bf39a519f27c67#main-key",algorithm="hs2019",headers="(request-target) host date digest",signature="NjivuB3qFMopxLw/JMdMw2pi7ao31lEZDLM+GuyRXrUHBOwGc5jZ0tO+wOgCNbZ2wVhJq7P6jk0WzEjRSAXvmntv5xlNVNFepwWhhmR9ti+HA836wr9VNhijfK/J4UaXTAbV14tCKmrmPLptDAAXz4kpWMXrCQD9OORnNUXZJShpBj+ahmMdQYOfcPFNBxy3qpfRlGhKHl10kl1ehlDXGcwG+yldJaxBQM5Rln4edyliI6Ij5l1RMo8Xq0NUlIo6ZSg2FdVWAQfGKeW/m5EP4N4wPRdItmJ2C2BR/+zzFW1fy5//FboYPzr/887exxtw4aVvF73m6QwOx+zDs8oJ+w=="
		Content-Length: 338

		{"@context":"application/activity+json","actor":"https://emdev.ddns.net/@64d68054a4bf39a519f27c67","id":"https://emdev.ddns.net/@64d68054a4bf39a519f27c67/pub/following/64fdf0c1a835dc65840d6ffb","object":"https://activitypub.academy/users/angusius_dukernen","to":"https://activitypub.academy/users/angusius_dukernen/inbox","type":"Follow"}`)

	keyFinder := func(_ string) (string, error) {
		return "-----BEGIN RSA PUBLIC KEY-----\nMIIBCgKCAQEAusAu2SkpV2zpD4Yhzwc4fV2RDMNLoTQp+gS2w5xUaLHwyPJckCZi\n5l4Aj9OvebwGXKicAUund8vMiRFeTeOlV6+UUK8/kV0WWbO2cWlvvqcOyBm4VKxz\nigHKGVD0HM2QmSAf4XyXRLuCbJjawUm36xZgGRgkG/YDFD+0YqQWVmjpwVSjiXc8\nx0GqV9LottkNeQf8YMgNEp8sW0fY7h2RmkdILNqJ7rX3UjE6MBXg+rlJqAGy/Zge\nuGfQIAe+bA53onjJNRonSLKpeYMSGKAKWONfHMnWScqBy7b79OOKLGIzh9v/3hk7\nEvKGydvXckxu9KW2u/0wFV7NVLbS5XEo0wIDAQAB\n-----END RSA PUBLIC KEY-----\n", nil
	}

	// Make a new request
	request, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rawHTTP)))
	require.Nil(t, err)

	// Verify the request
	_, err = Verify(request, keyFinder, VerifierIgnoreTimeout())
	require.Nil(t, err)
}

// Test_Emissary_Digest confirms DigestSHA256 matches the Digest header of the captured Emissary request
func Test_Emissary_Digest(t *testing.T) {
	expected := "gH5r8XfomFHwvjZUZAqtbE1MpJkmbgCZ3cQjYGt7ppA="
	body := `{"@context":"application/activity+json","actor":"https://emdev.ddns.net/@64d68054a4bf39a519f27c67","id":"https://emdev.ddns.net/@64d68054a4bf39a519f27c67/pub/following/64fdf0c1a835dc65840d6ffb","object":"https://activitypub.academy/users/angusius_dukernen","to":"https://activitypub.academy/users/angusius_dukernen/inbox","type":"Follow"}`

	digestBytes := sha256.Sum256([]byte(body))
	digest := base64.StdEncoding.EncodeToString(digestBytes[:])

	require.Equal(t, expected, digest)
	require.Equal(t, expected, DigestSHA256([]byte(body)))
}

// Test_Emissary_FailedSignature_1 confirms that a captured delivery between two local Emissary servers verifies
func Test_Emissary_FailedSignature_1(t *testing.T) {

	rawHTTP := removeTabs(
		`POST /@65aff7093de7b36dd13a2a4c/pub/inbox HTTP/1.1
		Host: localhost
		Digest: SHA-256=gTInMhzzf8laA9eqkQXxalgAOO0QgR7QB8psmUITlbg=
		Signature: keyId="http://127.0.0.1/65db7d011b663b1134c665dd#main-key",algorithm="rsa-sha256",headers="(request-target) host date digest",signature="ogWo+9hK03etvHkGIB6jjsZgo34DiGMqD1258bX7E6rJUkAeXN9LlBluu+qAAuLf/T2t2EQdRzNB+BAUJnqW2hmsg5dQoa46YaUrR040sfh5Vc1XNr5TQydatiQQDVb01NgOIJsnikKe+Go21lwoJDZriMJFS8mQP+BCFOorSrghqVCkp/yywDoGi0YU7WSVut3HOc2wG5ngUSZ+le+WTHz0+FUzuzxzYP4GlufHFnjgplsrLObFLXdLbm6akn4pidySXKLzpUEi8aLH0fQtTog6OiD0SulOyFo6TIsCNpPqFbNBVQ4VIAHvFpfkDP1xl85dDI/DjzDBP6ess1sf8w=="
		User-Agent: Go-http-client/1.1
		Content-Type: application/activity+json
		Accept: application/activity+json
		Date: Sun, 25 Feb 2024 20:58:55 GMT
		Accept-Encoding: gzip
		Content-Length: 636

		{"@context":"https://www.w3.org/ns/activitystreams","actor":"http://127.0.0.1/@65aff71e3de7b36dd13a2a59","object":{"actor":"http://127.0.0.1/@65aff71e3de7b36dd13a2a59","attachment":[],"attributedTo":"http://127.0.0.1/@65aff71e3de7b36dd13a2a59","context":"http://127.0.0.1/65db800832c5ad74da313f9f","id":"http://127.0.0.1/65db800832c5ad74da313f9f","name":"First Task!","published":"2024-02-25T17:59:36Z","to":["https://www.w3.org/ns/activitystreams#Public"],"type":"Note","url":"http://127.0.0.1/65db800832c5ad74da313f9f"},"published":"Sun, 25 Feb 2024 20:58:55 GMT","to":["https://www.w3.org/ns/activitystreams#Public"],"type":"Update"}
		`)

	keyFinder := func(_ string) (string, error) {
		return "-----BEGIN RSA PUBLIC KEY-----\nMIIBCgKCAQEAwxZUnNE5IuP3g2xgDhXTWhhpIscBPJHMHgRRR6IxLy4NW+Ll4Q95\nnJkjXiQsJXv1617FJw1aL+jKAkw++Rp0yawFG+RLomBP6tLL9vhfYm3JQ2BBbtdb\nbmaY/b9l03peaqCLFNsl0ew40avvXxHEXmBB39yZhlnE0vCUzLNxQfQoDDlYyBI+\nfeVE4bIlkQzz5aMFtpnM3MQYhlcOnRkZpeibDIZ26H6ZHhuN6mr55g4HbZ5e7lCj\nvt+w251ohmePz7fwamkvi13r5W6xaY038/JkztmZL2Ncf99E//7A/1COXtjELE00\nrt8/JuRP22moIn7a3PFgI0K7GOETuGf/bwIDAQAB\n-----END RSA PUBLIC KEY-----\n", nil
	}

	// Make a new request
	request, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rawHTTP)))
	require.Nil(t, err)

	// Verify the request
	_, err = Verify(request, keyFinder, VerifierIgnoreTimeout())
	require.Nil(t, err)
}

// Test_Emissary_FailedSignature_2 confirms that a second captured delivery between two local
// Emissary servers, with its headers in a different order, verifies
func Test_Emissary_FailedSignature_2(t *testing.T) {

	rawHTTP := removeTabs(
		`POST /@65aff7093de7b36dd13a2a4c/pub/inbox HTTP/1.1
		Host: localhost
		User-Agent: Go-http-client/1.1
		Content-Length: 645
		Accept: application/activity+json
		Content-Type: application/activity+json
		Signature: keyId="http://127.0.0.1/65db7d011b663b1134c665dd#main-key",algorithm="rsa-sha256",headers="(request-target) host date digest",signature="GKSHg14zsxwqk+RYaftKO4D8k4RRhne+qd6GQRbt5SQjdET1IPFWqf0qtahV52ayjqXNpc9wACW7dwqqUn8mrqY59Yb9cgwql+iPZ+4MKnmgYLQqb9l9degYYs4eVaBe3HO5HguvZL5S0TFW5bfrGPPzLOT19FYOVzstXbvdFvZfaojn34PXS5nfj8ulSsar+22pikA+iSJ0nLCzOTuljpSVhOwnTNlbzERn1oESMfHxTlmnoTdm/xdm3cGpMmdays3afYkTJKpLVGqHBY2SL9cm1EXeZX6owUTIjzDeq6FXxfkxFnsMWnPMVKjeVA9LZ/Tjh89Sdr0uoXeoWXocIA=="
		Date: Sun, 25 Feb 2024 23:31:27 GMT
		Digest: SHA-256=A/HwbNCsGsAMz8/3iC/NIK3izY1ob6SQUeOcTKJ60tg=
		Accept-Encoding: gzip
		
		{"@context":"https://www.w3.org/ns/activitystreams","actor":"http://127.0.0.1/@65aff71e3de7b36dd13a2a59","object":{"actor":"http://127.0.0.1/@65aff71e3de7b36dd13a2a59","attachment":[],"attributedTo":"http://127.0.0.1/@65aff71e3de7b36dd13a2a59","context":"http://127.0.0.1/65dbcdcf549a6a4c0dbd1d5a","id":"http://127.0.0.1/65dbcdcf549a6a4c0dbd1d5a","name":"A new task to reject","published":"2024-02-25T23:31:27Z","to":["https://www.w3.org/ns/activitystreams#Public"],"type":"Note","url":"http://127.0.0.1/65dbcdcf549a6a4c0dbd1d5a"},"published":"Sun, 25 Feb 2024 23:31:27 GMT","to":["https://www.w3.org/ns/activitystreams#Public"],"type":"Create"}`)

	keyFinder := func(_ string) (string, error) {
		return "-----BEGIN RSA PUBLIC KEY-----\nMIIBCgKCAQEAwxZUnNE5IuP3g2xgDhXTWhhpIscBPJHMHgRRR6IxLy4NW+Ll4Q95\nnJkjXiQsJXv1617FJw1aL+jKAkw++Rp0yawFG+RLomBP6tLL9vhfYm3JQ2BBbtdb\nbmaY/b9l03peaqCLFNsl0ew40avvXxHEXmBB39yZhlnE0vCUzLNxQfQoDDlYyBI+\nfeVE4bIlkQzz5aMFtpnM3MQYhlcOnRkZpeibDIZ26H6ZHhuN6mr55g4HbZ5e7lCj\nvt+w251ohmePz7fwamkvi13r5W6xaY038/JkztmZL2Ncf99E//7A/1COXtjELE00\nrt8/JuRP22moIn7a3PFgI0K7GOETuGf/bwIDAQAB\n-----END RSA PUBLIC KEY-----\n", nil
	}

	// Make a new request
	request, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rawHTTP)))
	require.Nil(t, err)

	// Verify the request
	_, err = Verify(request, keyFinder, VerifierIgnoreTimeout())
	require.Nil(t, err)
}
