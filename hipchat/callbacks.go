package hipchat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	gorillaMux "github.com/gorilla/mux"
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
	Tokens                map[uint32]string
}

// NewIntegration returns a pointer to a Integration that uses the provided Store.
func NewIntegration(store Store) *Integration {
	c := Integration{
		Store: store,
		installationCallbacks: make([]func(), 0),
		updatedCallbacks:      make([]func(), 0),
		removedCallbacks:      make([]func(), 0),
		Tokens:                make(map[uint32]string),
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
func (c *Integration) GetHandler() http.Handler {
	return c.handler
}

// AddInstallationCallback adds a callback that will be called when the integration is installed.
func (c *Integration) AddInstallationCallback(callback func()) {
	c.installationCallbacks = append(c.installationCallbacks, callback)
}

// AddUpdatedCallback adds a callback that will be called when an installation is updated.
func (c *Integration) AddUpdatedCallback(callback func()) {
	c.updatedCallbacks = append(c.updatedCallbacks, callback)
}

// AddRemovedCallback adds a callback that will be called when the integration is uninstalled.
func (c *Integration) AddRemovedCallback(callback func()) {
	c.removedCallbacks = append(c.removedCallbacks, callback)
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

func (c *Integration) CompleteInstallation(i *InstallRecord) {
	log.Println("Completing installation")
	client := NewClient("")
	credentials := ClientCredentials{i.OAuthID, i.OAuthSecret}
	// TODO: Hard-coded, but should be stored away when descriptor is generated.
	token, _, err := client.GenerateToken(credentials, []string{})
	if err != nil {
		log.Printf("Error requesting token: %v", err)
		return
	}
	log.Printf("Token obtained: %v", token)
	c.Tokens[token.GroupID] = token.AccessToken

	for _, callback := range c.installationCallbacks {
		go callback()
	}
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
