package main

import (
	"strings"
	"unicode"
)

const (
	riskNone     int16 = 0
	riskUsername int16 = 1 // orange: password contains the username
	riskWeak     int16 = 2 // red: very bad password
)

// passwordRisk is evaluated when a password is created, updated or imported.
// Duplicates (yellow) are derived at read time from fingerprints, so they
// update automatically when any record sharing the password changes.
func passwordRisk(password, username string) int16 {
	if isWeak(password) {
		return riskWeak
	}
	if u := strings.ToLower(strings.TrimSpace(username)); len(u) >= 3 && strings.Contains(strings.ToLower(password), u) {
		return riskUsername
	}
	return riskNone
}

func isWeak(pw string) bool {
	if len([]rune(pw)) < 8 {
		return true
	}
	if commonPasswords[strings.ToLower(pw)] {
		return true
	}
	var lower, upper, digit, other bool
	distinct := map[rune]bool{}
	for _, r := range pw {
		distinct[r] = true
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			other = true
		}
	}
	if len(distinct) <= 3 {
		return true
	}
	classes := 0
	for _, c := range []bool{lower, upper, digit, other} {
		if c {
			classes++
		}
	}
	return classes == 1 && len([]rune(pw)) < 12
}

var commonPasswords = func() map[string]bool {
	m := map[string]bool{}
	for _, p := range strings.Fields(`
123456 123456789 12345678 1234567890 password password1 password123 qwerty qwerty123 qwertyuiop
111111 1234567 123123 abc123 abcd1234 iloveyou admin admin123 welcome welcome1 letmein monkey
dragon football baseball sunshine princess master shadow superman batman trustno1 starwars
passw0rd p@ssw0rd p@ssword 1q2w3e4r 1qaz2wsx zaq12wsx asdfghjkl qazwsxedc 987654321 changeme
secret secret123 login hello123 freedom whatever michael jennifer jordan23 hunter2 charlie
liverpool chelsea arsenal computer internet samsung google mustang harley ranger buster soccer
hockey killer pepper ginger summer winter spring autumn flower lovely loveme blink182 matrix
cheese chocolate cookie purple orange yellow banana 00000000 11111111 12341234 88888888 66666666
nextcloud passman default guest root toor administrator`) {
		m[p] = true
	}
	return m
}()
