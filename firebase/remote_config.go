package firebase

import (
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

const firebaseURL = "https://firebaseremoteconfig.googleapis.com/v1/projects"

// RemoteConfigResponse represents the full response from Firebase Remote Config API.
type RemoteConfig struct {
	Parameters map[string]Parameter `json:"parameters"`
	Version    Version              `json:"version"`
	Conditions []Condition          `json:"conditions,omitempty"`
}

// Parameter represents a single Remote Config parameter.
type Parameter struct {
	DefaultValue DefaultValue `json:"defaultValue"`
	Description  string       `json:"description,omitempty"`
	ValueType    string       `json:"valueType"`
}

// DefaultValue represents the default value of a parameter.
type DefaultValue struct {
	Value string `json:"value"`
}

// Version represents metadata about the configuration version.
type Version struct {
	VersionNumber  string     `json:"versionNumber"`
	UpdateTime     time.Time  `json:"updateTime"`
	UpdateUser     UpdateUser `json:"updateUser"`
	UpdateOrigin   string     `json:"updateOrigin"`
	UpdateType     string     `json:"updateType"`
	Rollbacksource string     `json:"rollbacksource"`
	IsLegacy       bool       `json:"isLegacy"`
}

// UpdateUser represents details about the user who updated the config.
type UpdateUser struct {
	Email string `json:"email"`
}

// Condition represents conditions in Firebase Remote Config.
type Condition struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	TagColor   string `json:"tagColor,omitempty"`
}

var remoteConfig RemoteConfig

func AllConfigs() RemoteConfig {
	return remoteConfig
}

func SetRemoteConfig(rf RemoteConfig) {
	remoteConfig = rf
}

func IsEnabled(name string) bool {
	v, ok := remoteConfig.Parameters[name]
	if !ok {
		return false
	}

	return v.DefaultValue.Value == "true"
}

func IsDisabled(name string) bool {
	return !IsEnabled(name)
}

func Value(name string) string {
	v, ok := remoteConfig.Parameters[name]
	if !ok {
		return ""
	}

	return v.DefaultValue.Value
}

func GetRemoteConfig(token oauth2.Token, projectID string) (RemoteConfig, error) {
	url := firebaseURL + "/" + projectID + "/remoteConfig"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return RemoteConfig{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return RemoteConfig{}, err
	}
	defer resp.Body.Close()

	config := RemoteConfig{}
	err = json.NewDecoder(resp.Body).Decode(&config)
	return config, err
}

func ListVersion(token oauth2.Token, projectID string) ([]Version, error) {
	client := &http.Client{}
	url := firebaseURL + "/" + projectID + "/remoteConfig:listVersions?pageSize=1"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	vers := struct {
		Versions      []Version `json:"versions"`
		NextPageToken string    `json:"nextPageToken"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&vers)

	return vers.Versions, err
}
