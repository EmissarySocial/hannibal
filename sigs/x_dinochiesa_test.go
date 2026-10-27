package sigs

import (
	"bufio"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Sample transactions from - https://dinochiesa.github.io/httpsig/
func TestVerify_Dino1(t *testing.T) {

	requestString := removeTabs(
		`GET /foo HTTP/1.1
		Host: example.org
		x-request-id: 00000000-0000-0000-0000-000000000004
		tpp-redirect-uri: https://www.sometpp.com/redirect/
		digest: SHA-256=TGGHcPGLechhcNo4gndoKUvCBhWaQOPgtoVDIpxc6J4=
		psu-id: 1337
		Signature: keyId="abcdefg-123", algorithm="rsa-sha256", headers="x-request-id tpp-redirect-uri digest psu-id", signature="XRUrq4Jm88DiL8EPbp2EP1033F1H0GXhOoO+GJtBee9Or7X8oMXabPdQIS1GCrAqGpXPK3Dod4M20RsshUJ+aDPhhTaDLIpu6veFjvN3ks6rMlrFjsHNM9IIeQGyFcDp8ByohxOwb7KxzOcQgrvAUPdtBj6HuMjMU0ymDRgxkIwM+joM6ptG38bpKntDLdfbktZRppM/GTsyPnd79u6eWCOXOwis7KyMHUFWDvZ3c5LnHTEG4jYynAuKW3sbc1tCxUtlrLrJyh0HhsUigrPGhLjGQPbHbUGej0AcowYBILXbe7CPhKJegEwmWeMNK/L1CZT5pmmh4aG3lKL/3BqGaw=="
		
		`)

	requestReader := bufio.NewReader(strings.NewReader(requestString))
	request, err := http.ReadRequest(requestReader)
	require.Nil(t, err)

	keyFinder := func(_ string) (string, error) {
		return removeTabs(`-----BEGIN PUBLIC KEY-----
		MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtHiuMUfXZ+H7ruDYDfNZ
		OYD1PChkSZJoHgoS/3qrue/O3QM7UEos9sWR2yQ3xH5VdLMx0jR9yaPQfe6bS0C5
		ZziR4FA3VQ2nVCSYZNbaZGEms81yXS6qMhE/kbIjbYBm5DFWKYIPllH2IXMSiaGA
		Wd9LQHI5or7m/tfzgYBIAgErztb9oz456GHPsiAJnkSbLYP+cRMobUn+NY/stYSK
		Nq/Q+Ld9Q5ewj5qg7ps1f9LQ5kEDRaZY5pXvMjk9qfGI02hvxprRtXmC/zSiOUI6
		yO5EHO6Yg4b8+/9sELdIuGqDRg7uINfgddMAF9EXif4MkFCgiJnvT0xro6M7mgLx
		lwIDAQAB
		-----END PUBLIC KEY-----`), nil
	}

	_, err = Verify(request, keyFinder, VerifierFields("x-request-id", "tpp-redirect-uri", "digest", "psu-id"), VerifierIgnoreTimeout(), VerifierIgnoreBodyDigest())
	require.Nil(t, err)
}

func TestVerify_Dino2(t *testing.T) {

	requestString := removeTabs(
		`GET /foo HTTP/1.1
		Host: example.org
		x-request-id: 00000000-0000-0000-0000-000000000004
		tpp-redirect-uri: https://www.sometpp.com/redirect/
		digest: SHA-256=TGGHcPGLechhcNo4gndoKUvCBhWaQOPgtoVDIpxc6J4=
		psu-id: 1337		
		Signature: keyId="abcdefg-123", algorithm="rsa-sha256", headers="x-request-id tpp-redirect-uri digest psu-id", signature="zNEg4c1B01I5NaimvAF+/ZY1gtHge38NPTungOyKZSr2drIjm1KbvZxMl7krrzpUkkzp1Kt0GDKbiv5bsqbI/j15wyhgJmuZF6QvDip29SyAZwO83MuUF2vH66MeXVR6wZ8RvNDwYRoDjwGQ8DOadmQfM3ew2ySDuUT+/FUliFHH+SMZZKH5Ee0x4tdmLopvKdIu39OSHh0AvwpRZhGqLDNdCG0VjLVKdgCKFwKsb9f84PtU47PW3kTlHxUUuQ5vjwazmARjvHKyXRpKBkGJJBUlNEHqCdt54pxTd8YsmJPbcmUMmmiMwEWFanyy2JX9i8JgFXTf6pAGX0FpBqMhjQ=="

		`)

	requestReader := bufio.NewReader(strings.NewReader(requestString))
	request, err := http.ReadRequest(requestReader)
	require.Nil(t, err)

	keyFinder := func(_ string) (string, error) {
		return removeTabs(
			`-----BEGIN PUBLIC KEY-----
			MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA4wCAxz92RHsFI6F6c1nP
			2HDkRJxo0Ub8QSomgeOdqPWZGQg0n2x7OCH5oJT9bur0mxiWLOiC607BmD8zaamE
			QTSgaz+VLfBcn5LQ73E+O8UB3tJr4k4JgD0eCmUmJ1nMNp+ArhgOZrYcbezt9BsE
			vR77YUlSXs6LnCVa5niGTRwmJMOeljP1lEIoUVRnOlWD9ZBCtApnZvHPLV6tQnpf
			36G7fMXXPINyg9lw/GmQWcI+PHqUDRYgea3u5Q1NLau1GZqP0vn+NyWMI9Ma3nZx
			Nz51N02SnsUepzH7TjUPPfPlHc1uItaQgCGBaJUAdMmQbaM+Ww69y4TXUZEW22kp
			hQIDAQAB
			-----END PUBLIC KEY-----`), nil
	}

	_, err = Verify(request, keyFinder, VerifierFields("x-request-id", "tpp-redirect-uri", "digest", "psu-id"), VerifierIgnoreTimeout(), VerifierIgnoreBodyDigest())
	require.Nil(t, err)
}

func TestSign_Dino(t *testing.T) {

	requestString := removeTabs(
		`GET /foo HTTP/1.1
		Host: example.org
		x-request-id: 00000000-0000-0000-0000-000000000004
		tpp-redirect-uri: https://www.sometpp.com/redirect/
		psu-id: 1337

		`)

	requestReader := bufio.NewReader(strings.NewReader(requestString))
	request, err := http.ReadRequest(requestReader)
	require.Nil(t, err)

	privateKeyPEM := removeTabs(`-----BEGIN PRIVATE KEY-----
	MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCuQ/0EvPOxk0Hp
	Yu92Es3hjJuhbLUC80WTdIv5HVqwubaMOvxPoTnWbqXBMbcv38oh5HB0b4RQQuoi
	R78cLx1410wE+uOtF+ViU6bhZhUOozE7xxISgF3Mi1k0Hzi5/EVNvyIS8gnsRmvq
	wtk0ntBwwJQqwdcKZI8kQwShujr4GkO8EHgIhT4MbV+lIHUr7mcRiBhC4GEiNLbv
	EfG+EJOquKacHyX+L+3Unnhnt81WlkcnVfY1d4j7ISDi2C46GuMudL3GKRBqJrZZ
	OTdUoljNjc5P8AHmeDcXXhHhlLidblX4+X7kGvvm1pRrewyq+mdYDk82iVqmc4cM
	bdfvH1dxAgMBAAECggEACVjhuy2UZ6+1nxpcrEllbCXx2hZ93g7k6jwb3uyVZvnL
	Ihnu2ymTc95C+0oaoJGBItDBPGmX4AM60kRlapJXYxovPGwlpqzrs5q3jor+cabM
	tv9eR4pFnblSu1I6ZXVz1S/9mKUNZbRASRsS8fjbxtR5jhKQIYFT0TbcCn21+IU/
	Xk9I75jTXwFyrs5vG+FdE2cJ6cbXDKQF/Tx66dQZJd8Ow7VBSXLJ0nZcFojyDXs4
	MntrZ5aygsE20EQPX3j541UrcShGtnHnFyL2KQF8Xn0SW7o9kCyNVKkvwMVT663i
	neCd5hraSZPGYGEgWdGc8zwg4y0Uefm4Om6NjXnXjQKBgQDoXyUYtDhwpbKZT9Ao
	unVqR1NqC+bDqS+rm8gu48KYx7z62rND4GKpSKlx8ilhuCDyYLZti3AymRDsokQH
	9ETB6AJy9+tirzR8l8eorSpYl6XPt4cso7b8pmgbpVgwfuYKJqGDTuIVxa4x5QMI
	7/+oHCVDoe6t06P17pigMZCR3QKBgQC//El2mwyXOzrAfRq0hkQshp2GvRCasEVS
	T7Ql+3UkyCHEWeqsQ+unGQsbHOgwkBLbpv18rwvTwXdykv1x9FG5nzdyhr8sAwj7
	JBmnRYal2qj+TOsra2tihCYpEhUxJZeotbAi2yvMVg+b/dkOWUS32nIhY0RrmqqO
	gIJckeNkpQKBgA7IJqr4o/J+h+r6ycodemSlXugLE8X0mES5ZzWcZX+kjSAEE41I
	093i8mx+NCW0OdxRTKmRSjTdydbTx7Id1tXi9Wzs2ntvm84lNZ1ETsJN+01IZn/v
	di+CQnMnxIFpQSb6KCIbPYSXC6q+37+MzN2b1L8FqRJDuVVmtSzTmle9AoGBALvl
	Yczn8MmuWVD83/8gjWZ6lX/CWJbcv+vQQAMQeNT33jx6uDfC/cb7tqfhgcnNp/c8
	F0lJVKz54zrKa6x0ruuZzT2UbVPY4JhS+5x/akm2mMDSbTOAnYe8yFBX90+zeBvR
	PkLO+K2y6PIF3sKxUZUTAbJ1oggiRpzTX0LUMZZVAoGBAI1kx5P6LcLN+JS+Xe/m
	MAat7Swyr4MPy/nwi/pNy0p3jABrzu24EYBeuf0yF5Qo0PYfsejbr6sa2fs8lpkx
	0xDM4SRfk/OhNlxg/8oMjVGKD6AIXLCyThAw+RFyTrkO2vUrTm29zYEAtaVVAfgc
	oUzfO75tQ/QHU3ZvtVnEERXh
	-----END PRIVATE KEY-----`)

	privateKey, err := DecodePrivatePEM(privateKeyPEM)
	require.Nil(t, err)

	err = Sign(request, "abcdefg-123", privateKey, SignerFields("x-request-id", "tpp-redirect-uri", "psu-id"))
	require.Nil(t, err)
	require.Equal(t, `keyId="abcdefg-123",algorithm="rsa-sha256",headers="x-request-id tpp-redirect-uri psu-id",signature="ik5wHHA4LP23hOw+tg4doz0150JzU3RFfPu7zX6i76wGLEUeh7jP+zbjpQrTHVnrcJEE0qTBmQySgZODxpMmOIZPGjOanTVV2FVypVmEuyoTT+yqHymgqlJt8SUEko+jhbboQ+o184HWP17ZuWH7+vBZoQWKN1nglZa5Bkef4/1JWmTrwTAmp9mWqfVJp1X5fzaM8gqXdO103KW/Znb/sK3fuMnLjZNNw7oaeSRUSFxv9i9hKgHSdUFV0YglxWvOtFhagHQ4gp9nKikjSIPUppdjlHB7/VVXVqAKvLRaomL7pWO4RVH18hLdPHLrzeezMJLOJnxgzB4i1KG01xu3zw=="`, request.Header.Get("Signature"))
}
