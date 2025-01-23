package firebase

import (
	"context"
	"encoding/json"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
)

type ServiceAccount struct {
	AuthURI    string `json:"auth_uri"`
	ProjectID  string `json:"project_id"`
	Email      string `json:"client_email"`
	PrivateKey string `json:"private_key"`
}

func ReadServiceAccount(filename string) (ServiceAccount, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return ServiceAccount{}, err
	}
	sa := ServiceAccount{}
	err = json.Unmarshal(b, &sa)
	return sa, err
}

func Authen(email, privateKey string) oauth2.TokenSource {
	cf := &jwt.Config{
		Email:      email,
		PrivateKey: []byte(privateKey),
		Scopes: []string{
			"https://www.googleapis.com/auth/firebase.remoteconfig",
		},
		TokenURL: google.JWTTokenURL,
	}
	tokenSource := cf.TokenSource(context.Background())

	return tokenSource
}

func GetFirebaseRemoteConfig(email, privateKey, projectID string) error {
	tokenSource := Authen(email, privateKey)
	token, err := tokenSource.Token()
	if err != nil {
		return err
	}

	conf, err := GetRemoteConfig(*token, projectID)
	if err != nil {
		return err
	}

	SetRemoteConfig(conf)
	return nil
}
