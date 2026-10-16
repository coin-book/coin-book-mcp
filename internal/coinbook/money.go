package coinbook

import (
	"strconv"
	"strings"
)

var currencyLabels = map[string]string{"CZK": "Kč", "EUR": "€", "USD": "$"}

var zeroDecimal = map[string]bool{"JPY": true, "KRW": true, "ISK": true}

// Format renders an amount for an agent to read aloud; twin of
// coin-book-api/internal/platform/money/format.go, pinned by testdata/format-vectors.json.
func Format(a Amount) string {
	value := a.Minor
	sign := ""
	if value < 0 {
		sign, value = "−", -value
	}
	exponent := 2
	if zeroDecimal[a.Currency] {
		exponent = 0
	}
	digits := strconv.FormatInt(value, 10)
	if len(digits) <= exponent {
		digits = strings.Repeat("0", exponent-len(digits)+1) + digits
	}
	text := group(digits[:len(digits)-exponent])
	if exponent > 0 {
		text += "," + digits[len(digits)-exponent:]
	}
	label, ok := currencyLabels[a.Currency]
	if !ok {
		label = a.Currency
	}
	return sign + text + " " + label
}

func group(whole string) string {
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteString(" ")
		}
		b.WriteRune(r)
	}
	return b.String()
}
