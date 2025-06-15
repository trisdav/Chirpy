package auth
import (
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
	"errors"
	"net/http"
	"strings"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn).UTC()),
		Subject: userID.String(),
	})

	tokenString, err := token.SignedString([]byte(tokenSecret))
	return tokenString, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	var id uuid.UUID
	if err != nil {
		return id, err
	} else if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {

		id,_ = uuid.Parse(claims.Subject)
	} else {
		return id,err //log.Fatal("unknown claims type, cannot proceed")
	}

	return id,nil
}

func HashPassword(password string) (string, error) {
	bP,err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost);
	return string(bP), err

}

func CheckPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash),[]byte(password));
}

func GetBearerToken(header http.Header) (string, error) {
	hstr := header.Get("Authorization");
	var err error
	if !strings.HasPrefix(hstr, "Bearer ") {
		err = errors.New("No bearer token in header")
	}
	return strings.TrimPrefix(hstr,"Bearer "), err
}