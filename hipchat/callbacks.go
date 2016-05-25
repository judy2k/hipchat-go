package hipchat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// InstallRecord represents the structure sent to /installed for unmarshalling.
type InstallRecord struct {
	CapabilitiesURL string `json:"capabilitiesUrl"`
	OAuthID         string `json:"oauthId"`
	OAuthSecret     string `json:"oauthSecret"`
	GroupID         uint64 `json:"groupId"`
	RoomID          uint64 `json:"roomId"`
}

// CallbackHandler stores state shared by callback handler functions
type CallbackHandler struct {
	store Store
}

// NewCallbackHandler returns a pointer to a CallbackHandler that uses the provided Store.
func NewCallbackHandler(store Store) *CallbackHandler {
	return &CallbackHandler{store}
}

func (c *CallbackHandler) handleInstalled(w http.ResponseWriter, r *http.Request) {
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

		err = c.store.SaveCredentials(&i)
		if err != nil {
			log.Printf("Error saving credentials to store: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "There was an error saving these credentials")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method %s not supported at %s", r.Method, r.URL.Path)
		return
	}

}

func (c *CallbackHandler) handleUpdated(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "TODO - handle %s callback", r.URL.Path)
}

func (c *CallbackHandler) handleRemoved(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		// TODO - validate request.
		oAuthID := mux.Vars(r)["oAuthId"]

		err := c.store.DeleteCredentials(oAuthID)
		if err != nil {
			log.Printf("Error deleting credentials credentials for %v: %v", oAuthID, err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "There was an error deleting these credentials")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method %s not supported at %s", r.Method, r.URL.Path)
	}
}

// CallbackHandler returns a HandlerMux with the callback endpoints configured.
func (c *CallbackHandler) CallbackHandler() http.Handler {
	mux := mux.NewRouter()
	mux.HandleFunc("/installed", c.handleInstalled)
	mux.HandleFunc("/installed/{oAuthId}", c.handleRemoved)
	mux.HandleFunc("/updated", c.handleUpdated)

	return mux
}
