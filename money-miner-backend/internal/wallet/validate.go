// Package wallet validates payout addresses per coin family (dossier 07:
// family regex + length; checksum where cheap). Addresses only — the app
// never asks for, stores, or transmits private keys or seed phrases.
//
// Depth per family is dossier open question #5: v0.1 ships format
// validation (regex + length + prefix); full checksum verification is
// tracked per family in docs/roadmap.md. A failed check is rejected with
// wallet_invalid and never stored.
package wallet

import (
	"fmt"
	"regexp"
	"strings"
)

type rule struct {
	family string
	re     *regexp.Regexp
	minL   int
	maxL   int
}

var rules = map[string]rule{
	// CryptoNote family (XMR/ZEPH): base58, 95-106 chars, starts 4/8 (XMR), ZEPH...
	"XMR":  {"cryptonote", regexp.MustCompile(`^[48][1-9A-HJ-NP-Za-km-z]{94,105}$`), 95, 106},
	"ZEPH": {"cryptonote", regexp.MustCompile(`^ZEPHYR[1-9A-HJ-NP-Za-km-z]{90,110}$`), 95, 120},
	// BTC-like base58/bech32
	"VTC":  {"btc-like", regexp.MustCompile(`^([3V][1-9A-HJ-NP-Za-km-z]{25,39}|vtc1[02-9ac-hj-np-z]{8,87})$`), 26, 90},
	"DASH": {"btc-like", regexp.MustCompile(`^[Xx7][1-9A-HJ-NP-Za-km-z]{25,39}$`), 26, 42},
	"DGB":  {"btc-like", regexp.MustCompile(`^([DS][1-9A-HJ-NP-Za-km-z]{25,39}|dgb1[02-9ac-hj-np-z]{8,87})$`), 26, 90},
	"BTG":  {"btc-like", regexp.MustCompile(`^([AG][1-9A-HJ-NP-Za-km-z]{25,39}|btg1[02-9ac-hj-np-z]{8,87})$`), 26, 90},
	// KAWPOW family
	"RVN":   {"raven", regexp.MustCompile(`^R[1-9A-HJ-NP-Za-km-z]{25,39}$`), 26, 42},
	"CLORE": {"raven", regexp.MustCompile(`^[CA][1-9A-HJ-NP-Za-km-z]{25,39}$`), 26, 42},
	// EVM-like 0x addresses (ETC/OCTA/FLUX run on eth-style addressing)
	"ETC":  {"evm", regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`), 42, 42},
	"OCTA": {"evm", regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`), 42, 42},
	"FLUX": {"zcash-t", regexp.MustCompile(`^t1[1-9A-HJ-NP-Za-km-z]{33}$`), 35, 35},
	"ZEC":  {"zcash", regexp.MustCompile(`^(t1[1-9A-HJ-NP-Za-km-z]{33}|t3[1-9A-HJ-NP-Za-km-z]{33}|zs1[02-9ac-hj-np-z]{75}|u1[02-9ac-hj-np-z]{20,})$`), 35, 160},
	"ARRR": {"zcash-sapling", regexp.MustCompile(`^zs1[02-9ac-hj-np-z]{75}$`), 78, 78},
	// Ergo: P2PK base58 starts with 9
	"ERG": {"ergo", regexp.MustCompile(`^9[1-9A-HJ-NP-Za-km-z]{50,60}$`), 51, 61},
	// Kaspa bech32: kaspa:q...
	"KAS": {"kaspa", regexp.MustCompile(`^kaspa:[02-9ac-hj-np-z]{61,63}$`), 67, 69},
	// XELIS bech32: xel:...
	"XEL": {"xelis", regexp.MustCompile(`^xel:[02-9ac-hj-np-z]{20,80}$`), 24, 84},
	// Grin: slatepack addresses grin1...
	"GRIN": {"grin", regexp.MustCompile(`^grin1[02-9ac-hj-np-z]{20,70}$`), 26, 76},
	// Beam: long base58-ish
	"BEAM": {"beam", regexp.MustCompile(`^[0-9a-fA-F]{64,80}$|^[1-9A-HJ-NP-Za-km-z]{60,90}$`), 60, 90},
	// Chia bech32: xch1...
	"XCH": {"chia", regexp.MustCompile(`^xch1[02-9ac-hj-np-z]{59}$`), 62, 62},
}

// Validate checks the address format for a currency symbol.
func Validate(symbol, address string) error {
	address = strings.TrimSpace(address)
	r, ok := rules[strings.ToUpper(symbol)]
	if !ok {
		return fmt.Errorf("no validation rule for currency %s", symbol)
	}
	if len(address) < r.minL || len(address) > r.maxL {
		return fmt.Errorf("%s address length %d outside [%d,%d]", symbol, len(address), r.minL, r.maxL)
	}
	if !r.re.MatchString(address) {
		return fmt.Errorf("address does not match the %s %s format", symbol, r.family)
	}
	return nil
}
