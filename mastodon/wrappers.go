package mastodon

import (
	"fmt"
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

func (mc MastodonClient) authorizedRequest(body url.Values) (*http.Request, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf(`https://%s/api/v1/statuses`, mc.homeserver), strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", mc.token))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request, nil
}
