package smartoltclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *Client) IsConfigured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

type SmartOLTONU struct {
	SerialNumber     string  `json:"serial_number"`
	Name             string  `json:"name"`
	OLTID            string  `json:"olt_id"`
	OLTName          string  `json:"olt_name"`
	Board            string  `json:"board"`
	Port             string  `json:"port"`
	ONU              string  `json:"onu"`
	PONPort          string  `json:"pon_port"` // Board/Port/ONU e.g. 15/13/3
	ZoneID           string  `json:"zone_id"`
	ZoneName         string  `json:"zone_name"`
	VLAN             int     `json:"vlan"`
	Mode             string  `json:"mode"`
	Status           string  `json:"status"` // Online, Offline, LOS
	LastStatusChange string  `json:"last_status_change"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	Address          string  `json:"address"`
	ODPName          string  `json:"odp_name,omitempty"`

	// Diagnostics (optional / cached)
	RxPowerDBM     *float64 `json:"rx_power_dbm,omitempty"`
	TxPowerDBM     *float64 `json:"tx_power_dbm,omitempty"`
	SignalQuality  string   `json:"signal_quality,omitempty"` // GOOD, WARNING, CRITICAL, LOS
	DistanceMeters int      `json:"distance_meters,omitempty"`
	WANIPv4        string   `json:"wan_ipv4,omitempty"`
	PPPoEUsername  string   `json:"pppoe_username,omitempty"`
	MatchedRegNo   string   `json:"matched_reg_no,omitempty"`
	MatchedCust    string   `json:"matched_customer,omitempty"`
	IsAttached     bool     `json:"is_attached"`
}

type SmartOLTSignal struct {
	SerialNumber   string  `json:"serial_number"`
	Status         string  `json:"status"` // Online, Offline, LOS
	SignalQuality  string  `json:"signal_quality"` // GOOD, WARNING, CRITICAL, LOS
	RxPowerDBM     float64 `json:"rx_power_dbm"`
	TxPowerDBM     float64 `json:"tx_power_dbm"`
	AttenuationDB  float64 `json:"attenuation_db"`
	DistanceMeters int     `json:"distance_meters"`
	DeviceType     string  `json:"device_type"`
	WANIPv4        string  `json:"wan_ipv4"`
	PPPoEUsername  string  `json:"pppoe_username"`
	RawDetails     string  `json:"raw_details,omitempty"`
}

func (c *Client) CheckHealth(ctx context.Context) (bool, error) {
	if !c.IsConfigured() {
		return false, fmt.Errorf("smartolt is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/system/get_olts", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Token", c.apiKey)
	req.Header.Set("User-Agent", "GoGiga-ISP-Onboarding/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("smartolt responded with HTTP %d: %s", resp.StatusCode, string(body))
	}
	return true, nil
}

// GetScopedONUs retrieves all ONUs visible under the API key (e.g. Zone GOGIGA)
func (c *Client) GetScopedONUs(ctx context.Context) ([]SmartOLTONU, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("smartolt is not configured")
	}

	// 1. Get ONU Details
	reqDetails, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/onu/get_all_onus_details", nil)
	if err != nil {
		return nil, err
	}
	reqDetails.Header.Set("X-Token", c.apiKey)
	reqDetails.Header.Set("User-Agent", "GoGiga-ISP-Onboarding/1.0")

	respDetails, err := c.httpClient.Do(reqDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch onu details: %w", err)
	}
	defer respDetails.Body.Close()

	if respDetails.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respDetails.Body)
		return nil, fmt.Errorf("smartolt error HTTP %d: %s", respDetails.StatusCode, string(body))
	}

	var detailsResp struct {
		Status bool `json:"status"`
		Onus   []struct {
			SN        string      `json:"sn"`
			Name      string      `json:"name"`
			OLTID     string      `json:"olt_id"`
			OLTName   string      `json:"olt_name"`
			Board     interface{} `json:"board"`
			Port      interface{} `json:"port"`
			ONU       interface{} `json:"onu"`
			ZoneID    interface{} `json:"zone_id"`
			ZoneName  string      `json:"zone_name"`
			VLAN      interface{} `json:"vlan"`
			Mode      string      `json:"mode"`
			Address   string      `json:"address"`
			ODBName   string      `json:"odb_name"`
			Latitude  interface{} `json:"latitude"`
			Longitude interface{} `json:"longitude"`
		} `json:"onus"`
	}

	if err := json.NewDecoder(respDetails.Body).Decode(&detailsResp); err != nil {
		return nil, fmt.Errorf("failed to parse onu details: %w", err)
	}

	// 2. Get Live Statuses
	statusMap := make(map[string]struct {
		Status           string
		LastStatusChange string
	})

	reqStatus, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/onu/get_onus_statuses", nil)
	if err == nil {
		reqStatus.Header.Set("X-Token", c.apiKey)
		reqStatus.Header.Set("User-Agent", "GoGiga-ISP-Onboarding/1.0")
		if respStatus, err := c.httpClient.Do(reqStatus); err == nil {
			defer respStatus.Body.Close()
			var stResp struct {
				Response []struct {
					SN               string `json:"sn"`
					Status           string `json:"status"`
					LastStatusChange string `json:"last_status_change"`
				} `json:"response"`
			}
			if err := json.NewDecoder(respStatus.Body).Decode(&stResp); err == nil {
				for _, st := range stResp.Response {
					statusMap[st.SN] = struct {
						Status           string
						LastStatusChange string
					}{
						Status:           st.Status,
						LastStatusChange: st.LastStatusChange,
					}
				}
			}
		}
	}

	result := make([]SmartOLTONU, 0, len(detailsResp.Onus))
	for _, o := range detailsResp.Onus {
		sn := strings.TrimSpace(o.SN)
		boardStr := fmt.Sprintf("%v", o.Board)
		portStr := fmt.Sprintf("%v", o.Port)
		onuStr := fmt.Sprintf("%v", o.ONU)
		ponPort := fmt.Sprintf("%s/%s/%s", boardStr, portStr, onuStr)

		vlanInt := 0
		if o.VLAN != nil {
			switch v := o.VLAN.(type) {
			case float64:
				vlanInt = int(v)
			case string:
				vlanInt, _ = strconv.Atoi(v)
			}
		}

		lat := toFloat(o.Latitude)
		lng := toFloat(o.Longitude)

		stInfo := statusMap[sn]
		status := stInfo.Status
		if status == "" {
			status = "Unknown"
		}

		result = append(result, SmartOLTONU{
			SerialNumber:     sn,
			Name:             o.Name,
			OLTID:            o.OLTID,
			OLTName:          o.OLTName,
			Board:            boardStr,
			Port:             portStr,
			ONU:              onuStr,
			PONPort:          ponPort,
			ZoneID:           fmt.Sprintf("%v", o.ZoneID),
			ZoneName:         o.ZoneName,
			VLAN:             vlanInt,
			Mode:             o.Mode,
			Status:           status,
			LastStatusChange: stInfo.LastStatusChange,
			Latitude:         lat,
			Longitude:        lng,
			Address:          o.Address,
			ODPName:          o.ODBName,
		})
	}

	return result, nil
}

// GetONUSignalDiagnostics fetches real-time optical power and diagnostic telemetry
func (c *Client) GetONUSignalDiagnostics(ctx context.Context, sn string) (*SmartOLTSignal, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("smartolt is not configured")
	}

	cleanSN := strings.TrimSpace(sn)
	if cleanSN == "" {
		return nil, fmt.Errorf("serial number cannot be empty")
	}

	url := fmt.Sprintf("%s/api/onu/get_onu_full_status_info/%s", c.baseURL, cleanSN)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Token", c.apiKey)
	req.Header.Set("User-Agent", "GoGiga-ISP-Onboarding/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call smartolt diagnostics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("smartolt diagnostics HTTP %d: %s", resp.StatusCode, string(body))
	}

	var rawResp struct {
		Status         bool   `json:"status"`
		FullStatusInfo string `json:"full_status_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse diagnostics response: %w", err)
	}

	text := rawResp.FullStatusInfo
	sig := &SmartOLTSignal{
		SerialNumber: cleanSN,
		RawDetails:   text,
		Status:       "Online",
	}

	// 1. Parse Rx Power: 1490nm Tx :... Rx:-17.372(dbm)
	reRx := regexp.MustCompile(`(?i)1490nm.*?Rx:?\s*([-\d.]+)\(dbm\)`)
	if m := reRx.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			sig.RxPowerDBM = val
		}
	}

	// 2. Parse Tx Power: 1310nm Rx :... Tx:2.219(dbm)
	reTx := regexp.MustCompile(`(?i)1310nm.*?Tx:?\s*([-\d.]+)\(dbm\)`)
	if m := reTx.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			sig.TxPowerDBM = val
		}
	}

	// 3. Parse Attenuation: 1490nm ... 24.239(dB)
	reAtt := regexp.MustCompile(`(?i)1490nm.*?([-\d.]+)\(dB\)`)
	if m := reAtt.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			sig.AttenuationDB = val
		}
	}

	// 4. Parse Distance: ONU Distance: 6237m
	reDist := regexp.MustCompile(`(?i)ONU Distance:\s*(\d+)m`)
	if m := reDist.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil {
			sig.DistanceMeters = val
		}
	}

	// 5. Parse IPv4: IPv4 address: 103.179.64.211
	reIP := regexp.MustCompile(`(?i)IPv4 address:\s*([\d.]+)`)
	if m := reIP.FindStringSubmatch(text); len(m) > 1 {
		sig.WANIPv4 = m[1]
	}

	// 6. Parse PPPoE Username: Username: 1400200002@gogiga.net.idStatus:
	reUser := regexp.MustCompile(`(?i)Username:\s*([a-zA-Z0-9_.@-]+?)(?:Status:|\s|$)`)
	if m := reUser.FindStringSubmatch(text); len(m) > 1 {
		sig.PPPoEUsername = m[1]
	}

	// 7. Parse Device Type: Type: ZTE-F672YV9.1
	reType := regexp.MustCompile(`(?i)Type:\s*([^\s\n\r]+)`)
	if m := reType.FindStringSubmatch(text); len(m) > 1 {
		sig.DeviceType = m[1]
	}

	// 8. Quality classification according to Telco Standards (AGENTS.md):
	// -15.0 s/d -23.9 dBm: NORMAL / GOOD
	// -24.0 s/d -26.9 dBm: WARNING / LEMAH
	// <= -27.0 dBm: CRITICAL / KRITIS
	// <= -35.0 dBm atau 0 dBm: LOS
	if sig.RxPowerDBM == 0 || sig.RxPowerDBM <= -35.0 {
		sig.SignalQuality = "LOS"
		sig.Status = "LOS"
	} else if sig.RxPowerDBM <= -27.0 {
		sig.SignalQuality = "CRITICAL"
	} else if sig.RxPowerDBM <= -24.0 {
		sig.SignalQuality = "WARNING"
	} else {
		sig.SignalQuality = "GOOD"
	}

	return sig, nil
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0.0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0.0
}
