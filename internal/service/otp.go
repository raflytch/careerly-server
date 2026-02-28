package service

import (
	"crypto/rand"
	"math/big"
)

const otpDigits = "0123456789"

func GenerateOTP(length int) (string, error) {
	otp := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(otpDigits))))
		if err != nil {
			return "", err
		}
		otp[i] = otpDigits[n.Int64()]
	}
	return string(otp), nil
}
