package ja3transport

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	tls "github.com/refraction-networking/utls"
)

// ErrExtensionNotExist is returned when an extension is not supported by the library
type ErrExtensionNotExist string

// Error is the error value which contains the extension that does not exist
func (e ErrExtensionNotExist) Error() string {
	return fmt.Sprintf("Extension does not exist: %s\n", string(e))
}

// extMap maps extension values to the TLSExtension object associated with the
// number. Some values are not put in here because they must be applied in a
// special way. For example, "10" is the SupportedCurves extension which is also
// used to calculate the JA3 signature. These JA3-dependent values are applied
// after the instantiation of the map.
var extMap = map[string]tls.TLSExtension{
	"0": &tls.SNIExtension{},
	"5": &tls.StatusRequestExtension{},
	// These are applied later
	// "10": &tls.SupportedCurvesExtension{...}
	// "11": &tls.SupportedPointsExtension{SupportedPoints: []byte{0}},
	"13": &tls.SignatureAlgorithmsExtension{
		SupportedSignatureAlgorithms: []tls.SignatureScheme{
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
		},
	},
	"16": &tls.ALPNExtension{
		AlpnProtocols: []string{"http/1.1"},
	},
	"18": &tls.SCTExtension{},
	"21": &tls.UtlsPaddingExtension{GetPaddingLen: tls.BoringPaddingStyle},
	"23": &tls.UtlsExtendedMasterSecretExtension{},
	"27": &tls.UtlsCompressCertExtension{
		Algorithms: []tls.CertCompressionAlgo{
			tls.CertCompressionBrotli,
			tls.CertCompressionZlib,
			tls.CertCompressionZstd,
		}},
	"28": &tls.FakeRecordSizeLimitExtension{},
	"35": &tls.SessionTicketExtension{},
	"41": nil, // &tls.FakePreSharedKeyExtension{},
	"43": &tls.SupportedVersionsExtension{Versions: []uint16{
		tls.VersionTLS13,
		tls.VersionTLS12,
		tls.VersionTLS11,
		tls.VersionTLS10}},
	"44": &tls.CookieExtension{},
	"45": &tls.PSKKeyExchangeModesExtension{
		Modes: []uint8{
			tls.PskModeDHE,
		}},
	"51": &tls.KeyShareExtension{KeyShares: []tls.KeyShare{
		{Group: tls.CurveID(tls.GREASE_PLACEHOLDER), Data: []byte{0}},
		{Group: tls.X25519},
	}},
	"13172": &tls.NPNExtension{},
	"17513": &tls.ApplicationSettingsExtension{},
	"65281": &tls.RenegotiationInfoExtension{
		Renegotiation: tls.RenegotiateOnceAsClient,
	},
}

// NewTransport creates an http.Transport which mocks the given JA3 signature when HTTPS is used
func NewTransport(ja3 string) (*http.Transport, error) {
	return NewTransportWithConfig(ja3, &tls.Config{})
}

// NewTransportWithConfig creates an http.Transport object given a utls.Config
func NewTransportWithConfig(ja3 string, config *tls.Config) (*http.Transport, error) {
	spec, err := stringToSpec(ja3)
	if err != nil {
		return nil, err
	}

	dialtls := func(network, addr string) (net.Conn, error) {
		dialConn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}

		config.ServerName = strings.Split(addr, ":")[0]

		uTLSConn := tls.UClient(dialConn, config, tls.HelloCustom)

		if err := uTLSConn.ApplyPreset(spec); err != nil {
			return nil, err
		}
		if err := uTLSConn.Handshake(); err != nil {
			return nil, err
		}
		return uTLSConn, nil
	}

	return &http.Transport{DialTLS: dialtls}, nil
}

// stringToSpec creates a ClientHelloSpec based on a JA3 string
func stringToSpec(ja3 string) (*tls.ClientHelloSpec, error) {
	tokens := strings.Split(ja3, ",")

	version := tokens[0]
	ciphers := strings.Split(tokens[1], "-")
	extensions := strings.Split(tokens[2], "-")
	curves := strings.Split(tokens[3], "-")
	if len(curves) == 1 && curves[0] == "" {
		curves = []string{}
	}
	pointFormats := strings.Split(tokens[4], "-")
	if len(pointFormats) == 1 && pointFormats[0] == "" {
		pointFormats = []string{}
	}

	// parse curves
	var targetCurves []tls.CurveID
	for _, c := range curves {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		targetCurves = append(targetCurves, tls.CurveID(cid))
	}
	extMap["10"] = &tls.SupportedCurvesExtension{Curves: targetCurves}

	// parse point formats
	var targetPointFormats []byte
	for _, p := range pointFormats {
		pid, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil, err
		}
		targetPointFormats = append(targetPointFormats, byte(pid))
	}
	extMap["11"] = &tls.SupportedPointsExtension{SupportedPoints: targetPointFormats}

	// build extenions list
	var exts []tls.TLSExtension
	for _, e := range extensions {
		te, ok := extMap[e]
		if !ok {
			return nil, ErrExtensionNotExist(e)
		}
		if te == nil {
			continue
		}
		exts = append(exts, te)
	}
	// build SSLVersion
	vid64, err := strconv.ParseUint(version, 10, 16)
	if err != nil {
		return nil, err
	}
	vid := uint16(vid64)

	// build CipherSuites
	var suites []uint16
	for _, c := range ciphers {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		suites = append(suites, uint16(cid))
	}
	_ = vid
	return &tls.ClientHelloSpec{
		// TLSVersMin:         vid,
		// TLSVersMax:         vid,
		// TLSVersMin:         tls.VersionTLS10,
		// TLSVersMax:         tls.VersionTLS13,
		CipherSuites:       suites,
		CompressionMethods: []byte{0},
		Extensions:         exts,
		GetSessionID:       sha256.Sum256,
	}, nil
}
