package auth

import (
	"testing"
	"github.com/google/uuid"
	"time"
	"fmt"
)

func TestMakeJWT(t *testing.T) {
	testId := uuid.New()
	tokenSecret := "temp1234"
	result,err := MakeJWT(testId, tokenSecret, time.Minute*60)
	if (err != nil ) {
		t.Errorf("Unexpected nil %v",err);
	}
	expected:="";
	fmt.Printf("%s",result)
    if (result == expected) {
        t.Errorf("AuthFunction(\"testMakeJWT\") = %v; want %v", result, expected)
    }
}

func TestValidateJWT(t *testing.T) {
	testId := uuid.New()
	tokenSecret := "temp1234"
	result,err := MakeJWT(testId, tokenSecret, time.Minute*60)
	if (err != nil ) {
		t.Errorf("Unexpected nil %v",err);
	}
	expected:="";
	fmt.Printf("%s",result)
    if (result == expected) {
        t.Errorf("AuthFunction(\"testMakeJWT\") = %v; want %v", result, expected)
    }
	
	rid,verr := ValidateJWT(result, tokenSecret);

	if (err != nil) {
		t.Errorf("Unexpected nil %v",verr)
	}

	if (rid != testId) {
		t.Errorf("AuthFunction(\"TestValidateJWT) = %v; want %v", rid, testId)
	}
	
}