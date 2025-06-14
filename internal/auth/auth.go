package auth
import (
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bP,err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost);
	return string(bP), err

}

func CheckPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash),[]byte(password));
}