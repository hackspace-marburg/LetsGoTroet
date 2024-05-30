package mastodon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type appsReply struct {
	Id           string   `json:"id"`
	Name         string   `json:"name"`
	Website      string   `json:"website"`
	Scopes       []string `json:"scopes"`
	RedirectUri  string   `json:"redirect_uri"`
	ClientId     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	VapidKey     string   `json:"vapid_key"`
}

type tokenReply struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	CreatedAt   int64  `json:"created_at"`
}

type status struct {
	Id          string            `json:"id"`
	Content     string            `json:"content"`
	Url         string            `json:"url"`
	Account     account           `json:"account"`
	Attachments []mediaattachment `json:"media_attachments"`
	ResponseTo  string            `json:"in_reply_to_id"`
	Reblogged   bool              `json:"reblogged"`
	Favorited   bool              `json:"favourited"`
}

type account struct {
	Id          string `json:"id"`
	Username    string `json:"username"`
	Account     string `json:"acct"`
	DisplayName string `json:"display_name"`
}

type mediaattachment struct {
	Id   string `json:"id"`
	Type string `json:"type"`
	Url  string `json:"url"`
}

type search struct {
	Accounts []account `json:"accounts"`
	Statuses []status  `json:"statuses"`
	// There also are hashtags. Currently not supported
}

func (mc MastodonClient) authorizedRequest(request *http.Request) *http.Request {
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", mc.token))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func (mc MastodonClient) executeRequest(request *http.Request) ([]byte, error) {
	authorized := mc.authorizedRequest(request)
	resp, err := mc.client.Do(authorized)
	if err != nil {
		return nil, fmt.Errorf("Error in client.Do: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %w", err)
	}
	return respBody, nil
}

func (mc MastodonClient) toggleTootBoost(toot *status, visibility string) (*status, error) {
	action := "reblog"
	if toot.Reblogged {
		action = "un" + action
	}
	body := url.Values{}
	if !toot.Reblogged {
		body = url.Values{
			"visibility": {visibility},
		}
	}
	request, err := http.NewRequest("POST", fmt.Sprintf(`https://%s/api/v1/statuses/%s/%s`, mc.homeserver, toot.Id, action), strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("Error building request for boost: %w", err)
	}
	respBody, err := mc.executeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("Error during boost request: %w", err)
	}
	var updatedToot status
	if err = json.Unmarshal(respBody, &updatedToot); err != nil {
		return nil, fmt.Errorf("Error unmarshaling boost response: %w", err)
	}
	return &updatedToot, nil
}

func (mc MastodonClient) toggleTootFave(toot *status) (*status, error) {
	action := "favourite"
	if toot.Reblogged {
		action = "un" + action
	}
	body := url.Values{}
	request, err := http.NewRequest("POST", fmt.Sprintf(`https://%s/api/v1/statuses/%s/%s`, mc.homeserver, toot.Id, action), strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("Error building request for favouriting: %w", err)
	}
	respBody, err := mc.executeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("Error during fave request: %w", err)
	}
	var updatedToot status
	if err = json.Unmarshal(respBody, &updatedToot); err != nil {
		return nil, fmt.Errorf("Error unmarshaling favouriting response: %w", err)
	}
	return &updatedToot, nil
}

func (mc MastodonClient) deleteToot(toot *status) error {
	request, err := http.NewRequest("DELETE", fmt.Sprintf(`https://%s/api/v1/statuses/%s`, mc.homeserver, toot.Id), strings.NewReader(""))
	// The request response is not used. It should be the deleted toot when the delete was successfull
	_, err = mc.executeRequest(request)
	if err != nil {
		return fmt.Errorf("Error during boost request: %w", err)
	}
	return nil
}

func (mc MastodonClient) getStatus(tootId string) (*status, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf(`https://%s/api/v1/statuses/%s`, mc.homeserver, tootId), strings.NewReader(""))
	respBody, err := mc.executeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("Error during status request: %w", err)
	}
	var toot status
	if err = json.Unmarshal(respBody, &toot); err != nil {
		return nil, fmt.Errorf("Error unmarshaling response %s , %w", string(respBody), err)
	}
	return &toot, nil
}

func (mc MastodonClient) search(content string) (*search, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf(`https://%s/api/v2/search?q=%s`, mc.homeserver, content), strings.NewReader(""))
	if err != nil {
		return nil, fmt.Errorf("Error building request for search: %w", err)
	}
	respBody, err := mc.executeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("Error during search request: %w", err)
	}
	var searchResult search
	if err = json.Unmarshal(respBody, searchResult); err != nil {
		return nil, fmt.Errorf("Error unmarshaling search response: %w", err)
	}
	return &searchResult, nil
}

func (mc MastodonClient) postStatus(body url.Values) (string, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf(`https://%s/api/v1/statuses`, mc.homeserver), strings.NewReader(body.Encode()))
	respBody, err := mc.executeRequest(request)
	if err != nil {
		return "", fmt.Errorf("Error during status post request: %w", err)
	}
	var posted status
	if err = json.Unmarshal(respBody, &posted); err != nil {
		return "", fmt.Errorf("Error unmarshaling response %s , %s", string(respBody), err)
	}
	return mc.storeMessage(posted)
}

func (mc MastodonClient) getOwnAccount() (*account, error) {
	// Be aware: This endpoint returns a CredentialAccount and *not* an Account.
	// The CredentialAccount has additional fields, currently unused in this adapter: source and role
	request, err := http.NewRequest("GET", fmt.Sprintf(`https://%s/api/v1/accounts/verify_credentials`, mc.homeserver), strings.NewReader(""))
	respBody, err := mc.executeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("Error during own account request: %w", err)
	}
	var user account
	if err = json.Unmarshal(respBody, &user); err != nil {
		return nil, fmt.Errorf("Error unmarshaling response %s , %w", string(respBody), err)
	}
	return &user, nil
}
