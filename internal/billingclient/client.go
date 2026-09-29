package billingclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

func New(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type PlanDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	DownloadKbps int64   `json:"download_kbps"`
	UploadKbps   int64   `json:"upload_kbps"`
	MonthlyPrice float64 `json:"monthly_price"`
	PackageGroup string  `json:"package_group,omitempty"`
	ClusterCode  string  `json:"cluster_code,omitempty"`
	ClusterArea  string  `json:"cluster_area,omitempty"`
}

type CreateCustomerRequest struct {
	PartnerID  *string `json:"partner_id,omitempty"`
	FullName   string  `json:"full_name"`
	Email      *string `json:"email,omitempty"`
	Phone      string  `json:"phone"`
	Notes      *string `json:"notes,omitempty"`
	Street     string  `json:"street"`
	City       string  `json:"city"`
	District   *string `json:"district,omitempty"`
	Province   *string `json:"province,omitempty"`
	PostalCode *string `json:"postal_code,omitempty"`
}

type CustomerResponse struct {
	ID             string `json:"id"`
	CustomerNumber string `json:"customer_number"`
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Status         string `json:"status"`
}

type CreateSubscriptionRequest struct {
	CustomerID        string  `json:"customer_id"`
	PlanID            string  `json:"plan_id"`
	BillingCycle      *string `json:"billing_cycle,omitempty"`
	AutoRenewal       *bool   `json:"auto_renewal,omitempty"`
	InitialAccessType *string `json:"initial_access_type,omitempty"`
	InitialUsername   *string `json:"initial_username,omitempty"`
	InitialPassword   *string `json:"initial_password,omitempty"`
	Notes             *string `json:"notes,omitempty"`
}

type SubscriptionResponse struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	PlanID     string `json:"plan_id"`
	Status     string `json:"status"`
}

func (c *Client) CheckHealth(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) FetchPlans(ctx context.Context, clusterOrGroup string) ([]PlanDTO, error) {
	reqURL := c.baseURL + "/api/v1/plans?visible=true"
	if clusterOrGroup != "" {
		reqURL += "&cluster=" + url.QueryEscape(clusterOrGroup)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch plans (status %d): %s", resp.StatusCode, string(body))
	}

	var rawResult struct {
		Data []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Description  string `json:"description"`
			DownloadKbps int64  `json:"download_kbps"`
			UploadKbps   int64  `json:"upload_kbps"`
			PackageGroup string `json:"package_group"`
			ClusterCode  string `json:"cluster_code"`
			ClusterArea  string `json:"cluster_area"`
			CurrentPrice *struct {
				MonthlyPrice int64 `json:"monthly_price"`
			} `json:"current_price"`
			MonthlyPrice float64 `json:"monthly_price"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		return nil, err
	}

	plans := make([]PlanDTO, 0, len(rawResult.Data))
	for _, rp := range rawResult.Data {
		price := rp.MonthlyPrice
		if rp.CurrentPrice != nil && rp.CurrentPrice.MonthlyPrice > 0 {
			price = float64(rp.CurrentPrice.MonthlyPrice)
		}
		plans = append(plans, PlanDTO{
			ID:           rp.ID,
			Name:         rp.Name,
			Description:  rp.Description,
			DownloadKbps: rp.DownloadKbps,
			UploadKbps:   rp.UploadKbps,
			MonthlyPrice: price,
			PackageGroup: rp.PackageGroup,
			ClusterCode:  rp.ClusterCode,
			ClusterArea:  rp.ClusterArea,
		})
	}

	return plans, nil
}

func (c *Client) CreateCustomer(ctx context.Context, custReq CreateCustomerRequest) (*CustomerResponse, error) {
	jsonBytes, err := json.Marshal(custReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/customers", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("gogigabill customer creation failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var envelope struct {
		Success bool             `json:"success"`
		Data    CustomerResponse `json:"data"`
		CustomerResponse
	}
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, err
	}
	if envelope.Data.ID != "" {
		return &envelope.Data, nil
	}
	if envelope.CustomerResponse.ID != "" {
		return &envelope.CustomerResponse, nil
	}
	return nil, fmt.Errorf("unexpected empty customer response: %s", string(bodyBytes))
}

func (c *Client) FindCustomerByPhone(ctx context.Context, phone string) (*CustomerResponse, error) {
	cleanPhone := strings.TrimSpace(phone)
	if cleanPhone == "" {
		return nil, nil
	}
	reqURL := c.baseURL + "/api/v1/customers?search=" + url.QueryEscape(cleanPhone)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var res struct {
		Data []CustomerResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if len(res.Data) > 0 {
		return &res.Data[0], nil
	}
	return nil, nil
}

func (c *Client) FindCustomerByQuery(ctx context.Context, query string) (*CustomerResponse, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return nil, nil
	}
	reqURL := c.baseURL + "/api/v1/customers?search=" + url.QueryEscape(cleanQuery)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var res struct {
		Data []CustomerResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	for _, cust := range res.Data {
		if strings.EqualFold(cust.Email, cleanQuery) || cust.Phone == cleanQuery || cust.CustomerNumber == cleanQuery || cust.ID == cleanQuery {
			return &cust, nil
		}
	}
	if len(res.Data) > 0 {
		return &res.Data[0], nil
	}
	return nil, nil
}

func (c *Client) CreateSubscription(ctx context.Context, subReq CreateSubscriptionRequest) (*SubscriptionResponse, error) {
	jsonBytes, err := json.Marshal(subReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/subscriptions", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("gogigabill subscription creation failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var envelope struct {
		Success bool                 `json:"success"`
		Data    SubscriptionResponse `json:"data"`
		SubscriptionResponse
	}
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, err
	}
	if envelope.Data.ID != "" {
		return &envelope.Data, nil
	}
	if envelope.SubscriptionResponse.ID != "" {
		return &envelope.SubscriptionResponse, nil
	}
	return nil, fmt.Errorf("unexpected empty subscription response: %s", string(bodyBytes))
}

func (c *Client) SuspendSubscription(ctx context.Context, subscriptionID string) error {
	reqURL := fmt.Sprintf("%s/api/v1/subscriptions/%s/suspend", c.baseURL, subscriptionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to suspend subscription (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) ReactivateSubscription(ctx context.Context, subscriptionID string) error {
	reqURL := fmt.Sprintf("%s/api/v1/subscriptions/%s/reactivate", c.baseURL, subscriptionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to reactivate subscription (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

type RadiusSessionDTO struct {
	RadAcctID        int64     `json:"radacctid"`
	AcctSessionID    string    `json:"acctsessionid"`
	Username         string    `json:"username"`
	GroupName        string    `json:"groupname"`
	NasIPAddress     string    `json:"nasipaddress"`
	NasPortID        *string   `json:"nasportid,omitempty"`
	AcctStartTime    time.Time `json:"acctstarttime"`
	AcctSessionTime  int64     `json:"acctsessiontime"`
	AcctInputOctets  int64     `json:"acctinputoctets"`
	AcctOutputOctets int64     `json:"acctoutputoctets"`
	CallingStationID string    `json:"callingstationid"`
	FramedIPAddress  *string   `json:"framedipaddress,omitempty"`
	IsActive         bool      `json:"is_active"`
}

func (c *Client) ListActiveRadiusSessions(ctx context.Context) ([]RadiusSessionDTO, error) {
	reqURL := c.baseURL + "/api/v1/radius/sessions?limit=500"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list radius sessions (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		Success bool               `json:"success"`
		Data    []RadiusSessionDTO `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) DisconnectRadiusSession(ctx context.Context, nasIP, username, framedIP, sessionID string) error {
	reqURL := c.baseURL + "/api/v1/radius/sessions/disconnect"
	payload := map[string]string{
		"nas_ip_address":  nasIP,
		"username":        username,
		"framed_ip":       framedIP,
		"acct_session_id": sessionID,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to disconnect session (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

