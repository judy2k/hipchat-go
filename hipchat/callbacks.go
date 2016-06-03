package hipchat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	gorillaMux "github.com/gorilla/mux"
	"github.com/dgrijalva/jwt-go"
)

// InstallRecord represents the structure sent to /installed for unmarshalling.
type InstallRecord struct {
	CapabilitiesURL string `json:"capabilitiesUrl"`
	OAuthID         string `json:"oauthId"`
	OAuthSecret     string `json:"oauthSecret"`
	GroupID         uint64 `json:"groupId"`
	RoomID          uint64 `json:"roomId"`
}

// Integration stores state shared by callback handler functions
type Integration struct {
	Store                 Store
	installationCallbacks []func()
	updatedCallbacks      []func()
	removedCallbacks      []func()
	handler               http.Handler
	tokens                map[string]string // Key is "groupid:roomid"
}

// NewIntegration returns a pointer to a Integration that uses the provided Store.
func NewIntegration(store Store) *Integration {
	c := Integration{
		Store: store,
		installationCallbacks: make([]func(), 0),
		updatedCallbacks:      make([]func(), 0),
		removedCallbacks:      make([]func(), 0),
		tokens:                make(map[string]string),
	}

	mux := gorillaMux.NewRouter()
	mux.Path("/installed").Methods("POST").HandlerFunc(c.handleInstalled)
	//mux.HandleFunc("/installed", c.handleInstalled)
	mux.Path("/installed/{oAuthId}").Methods("DELETE").HandlerFunc(c.handleRemoved)
	mux.HandleFunc("/updated", c.handleUpdated)

	c.handler = mux

	return &c
}

// GetHandler obtains an http.Handler that should be attached to the http server
func (i *Integration) GetHandler() http.Handler {
	return i.handler
}

// AddInstallationCallback adds a callback that will be called when the integration is installed.
func (i *Integration) AddInstallationCallback(callback func()) {
	i.installationCallbacks = append(i.installationCallbacks, callback)
}

// AddUpdatedCallback adds a callback that will be called when an installation is updated.
func (i *Integration) AddUpdatedCallback(callback func()) {
	i.updatedCallbacks = append(i.updatedCallbacks, callback)
}

// AddRemovedCallback adds a callback that will be called when the integration is uninstalled.
func (i *Integration) AddRemovedCallback(callback func()) {
	i.removedCallbacks = append(i.removedCallbacks, callback)
}

func (c *Integration) handleInstalled(w http.ResponseWriter, r *http.Request) {
	// Note - this URL receives a DELETE request at /installed/oauth_id when the add-on is removed.

	if r.Method == "POST" {
		// TODO - validate request.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading installation data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "An unknown error occurred.")
			return
		}
		var i InstallRecord
		err = json.Unmarshal(body, &i)
		if err != nil {
			log.Printf("Error deserializing installation data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "There was an error deserializing the data.")
			return
		}

		err = c.Store.SaveCredentials(&i)
		if err != nil {
			log.Printf("Error saving credentials to Store: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "There was an error saving these credentials")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")

		go c.CompleteInstallation(&i)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method %s not supported at %s", r.Method, r.URL.Path)
		return
	}

}

func (i *Integration) CompleteInstallation(record *InstallRecord) {
	log.Println("Completing installation")

	_, err := i.getToken(record)
	if err != nil {
		log.Printf("Error requesting token: %v", err)
		return
	}

	for _, callback := range i.installationCallbacks {
		go callback()
	}
}

// getToken requests a token from HipChat and then caches the result
func (i *Integration) getToken(credentials *InstallRecord) (string, error) {
	client := NewClient("")
	// TODO: Hard-coded, but should be stored away when descriptor is generated.
	token, _, err := client.GenerateToken(ClientCredentials{credentials.OAuthID, credentials.OAuthSecret}, []string{})
	if err != nil {
		return "", err
	}
	log.Printf("Token obtained: %v", token)
	
	key := fmt.Sprintf("%v:%v", credentials.GroupID, credentials.RoomID)
	i.tokens[key] = token.AccessToken

	return token.AccessToken, nil
}

func (c *Integration) handleUpdated(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "TODO - handle %s callback", r.URL.Path)
	for _, callback := range c.updatedCallbacks {
		go callback()
	}
}

type Capabilities struct {
	OAuth2Provider Provider `json:"oauth2Provider"`
}

type Provider struct {
	AuthorizationURL string `json:"authorizationUrl"`
	TokenURL         string `json:"tokenUrl"`
}

func (c *Integration) getCapabilities(url string) (*Capabilities, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	capabilities := &Capabilities{}
	err = json.Unmarshal(data, capabilities)
	if err != nil {
		return nil, err
	}

	return capabilities, nil
}

func (c *Integration) handleRemoved(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		// TODO - validate request.
		oAuthID := gorillaMux.Vars(r)["oAuthId"]

		err := c.Store.DeleteCredentials(oAuthID)
		if err != nil {
			log.Printf("Error deleting credentials credentials for %v: %v", oAuthID, err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "There was an error deleting these credentials")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
		for _, callback := range c.removedCallbacks {
			go callback()
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method %s not supported at %s", r.Method, r.URL.Path)
	}
}

func (i *Integration) GetTokenForRoom(roomID uint32) (string, error) {
	// TODO: Handle token expiry
	groupID, err := i.Store.GetGroupID(roomID)
	if err != nil {
		return "", nil
	}
	
	key := fmt.Sprintf("%v:%v", groupID, roomID)
	
	token, exists := i.tokens[key]
	if !exists {
		credentials, err := i.Store.GetCredentials(groupID, roomID)
		if err != nil {
			return "", err
		}
		return i.getToken(credentials)
	}
	return token, nil
}

type SignedParams struct {
	RoomID uint32
	UserTimezone string
}

func (sp SignedParams) String() string {
	return fmt.Sprintf("SignedParams<RoomID: %v, Timezone: \"%v\">", sp.RoomID, sp.UserTimezone)
} 

func NewSignedParams(token *jwt.Token) (*SignedParams, error) {
	result := &SignedParams{}
	
	switch context := token.Claims["context"].(type) {
	case map[string]interface{}:
		if err := extractType(context, "room_id", &result.RoomID); err != nil {
			return nil, fmt.Errorf("Error extracting room_id: %v", err)
		}
		if err := extractType(context, "user_tz", &result.UserTimezone); err != nil {
			return nil, fmt.Errorf("Error extracting user_tz: %v", err)
		}
	default:
		return nil, fmt.Errorf("context of wrong type: %t", context)
	}
	
	return result, nil
}

func extractType(dict map[string]interface{}, key string, dest interface{}) error {
	if dict[key] == nil {
		return fmt.Errorf("Missing signed parameter \"%v\"", key)
	}
	switch d := dest.(type) {
	case *string:
		switch v := dict[key].(type) {
		case string:
			*d = v
			return nil
		}
	case *uint32:
		switch v := dict[key].(type) {
		case float32, float64:
			*d = uint32(v.(float64))
			return nil
		}
	}
	return fmt.Errorf("Type mismatch for signed param %v dest: %t, source: %t", key, dest, dict[key])
}

// ParseTokenFromRequest extracts and validates a JWT token from the request.
func (i *Integration) ParseSignedParams(req *http.Request) (*SignedParams, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		
		// Look up oauth secret with the iss string
		switch oauthID := token.Claims["iss"].(type) {
		case string:
			secret, err := i.Store.GetOAuthSecret(oauthID)
			if err != nil {
				return nil, err
			}
			
        return []byte(secret), nil
		default:
			return nil, fmt.Errorf("iss header of wrong type: %t", oauthID)
		}
	}
	
	// Look for an Authorization header
	if ah := req.Header.Get("Authorization"); ah != "" {
		prefix := "JWT "
		if strings.HasPrefix(strings.ToUpper(ah), prefix) {
			return parse(ah[len(prefix):], keyFunc)
		}
	}

	// Look for "signed_request" parameter
	req.ParseMultipartForm(10e6)
	if tokStr := req.Form.Get("signed_request"); tokStr != "" {
		return parse(tokStr, keyFunc)
	}

	return nil, jwt.ErrNoTokenInRequest
}

func parse(tokenStr string, keyFunc func(token *jwt.Token) (interface{}, error)) (*SignedParams, error) {
	token, err := jwt.Parse(tokenStr, keyFunc)
	if err != nil {
		return nil, err
	}
	return NewSignedParams(token)
}