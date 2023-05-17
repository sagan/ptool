This is a fork of https://github.com/CUCyber/ja3transport ,
With some modifications to make it work with latest utls 
( https://github.com/refraction-networking/utls )

It's used to manually craft "ja3" to resist the TLS ClientHello fingerprinting.
See https://scrapfly.io/web-scraping-tools/ja3-fingerprint for more details.

It's just a workaround at this time as the ja3transport
library has not been updated in recently.

utls example ClientHello TLSExtension

```
var DEFAULT_EXTS = []tls.TLSExtension{
	&tls.RenegotiationInfoExtension{
		Renegotiation: tls.RenegotiateOnceAsClient,
	}, // 65281
	&tls.StatusRequestExtension{}, // 5
	&tls.KeyShareExtension{KeyShares: []tls.KeyShare{
		{Group: tls.CurveID(tls.GREASE_PLACEHOLDER), Data: []byte{0}},
		{Group: tls.X25519},
	}}, // 51
	&tls.SNIExtension{},                 // 0
	&tls.ApplicationSettingsExtension{}, // 17513
	&tls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []tls.SignatureScheme{
		tls.ECDSAWithP256AndSHA256,
		tls.ECDSAWithP384AndSHA384,
		tls.ECDSAWithP521AndSHA512,
		tls.PSSWithSHA256,
		tls.PSSWithSHA384,
		tls.PSSWithSHA512,
		tls.PKCS1WithSHA256,
		tls.PKCS1WithSHA384,
		tls.PKCS1WithSHA512,
		tls.ECDSAWithSHA1,
		tls.PKCS1WithSHA1,
	}}, // 13
	&tls.UtlsCompressCertExtension{
		Algorithms: []tls.CertCompressionAlgo{
			tls.CertCompressionBrotli,
			tls.CertCompressionZlib,
			tls.CertCompressionZstd,
		},
	}, // 27
	&tls.ALPNExtension{AlpnProtocols: []string{"http2", "http/1.1"}}, // 16
	&tls.SCTExtension{}, // 18
	&tls.SupportedCurvesExtension{Curves: []tls.CurveID{tls.X25519, tls.CurveP256}}, // 10
	&tls.UtlsExtendedMasterSecretExtension{},                                        // 23
	&tls.PSKKeyExchangeModesExtension{Modes: []uint8{1}},                            // 45. pskModeDHE
	&tls.SessionTicketExtension{},                                                   // 35
	&tls.SupportedVersionsExtension{Versions: []uint16{
		tls.VersionTLS13,
		tls.VersionTLS12,
		tls.VersionTLS11,
		tls.VersionTLS10}}, // 43
	&tls.SupportedPointsExtension{SupportedPoints: []byte{0}}, // 11. uncompressed
	// &tls.FakePreSharedKeyExtension{},                          // 41
}
```

Presets

```
// ChromeAuto mocks Chrome 78
var ChromeAuto = Browser{
	JA3:       "769,47–53–5–10–49161–49162–49171–49172–50–56–19–4,0–10–11,23–24–25,0",
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36",
}

// SafariAuto mocks Safari 604.1
var SafariAuto = Browser{
	JA3:       "771,4865-4866-4867-49196-49195-49188-49187-49162-49161-52393-49200-49199-49192-49191-49172-49171-52392-157-156-61-60-53-47-49160-49170-10,65281-0-23-13-5-18-16-11-51-45-43-10-21,29-23-24-25,0",
	UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_1_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Mobile/15E148 Safari/604.1",
}
```