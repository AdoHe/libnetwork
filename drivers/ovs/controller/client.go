package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
)

const (
	base_path   = "/api/baymax/"
	api_version = "v1"

	create_network_action = "create_subnet"
	delete_network_action = "delete_subnet"
	list_network_action   = "list_network"
	request_ip_action     = "request_address"
	release_ip_action     = "release_address"
)

// The core network controller client.
type Client struct {
	Url string
	*http.Client
}

func NewClient(url string) (*Client, error) {
	if !isValidUrl(url) {
		return nil, fmt.Errorf("controller url must be in http://<ip>:<port> format")
	}

	return &Client{
		Url:    url,
		Client: http.DefaultClient,
	}, nil
}

// isValidUrl returns true if url has format: http://<ip>:<port>,
// otherwise returns false.
func isValidUrl(url string) bool {
	if url == "" {
		return false
	}
	// TODO: regex check
	return true
}

func (c *Client) CreateNetwork(networkName, cidr, gw string, id int) error {
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return ErrBadCIDRFormat(cidr)
	}
	if ip := net.ParseIP(gw); ip == nil {
		return ErrInvalidGWAddr(gw)
	}

	req := NewCreateNetworkRequest(networkName, cidr, gw, id)
	b, _ := json.Marshal(req)
	returnedObj := &StandardResponse{}
	err := c.sendRequest(create_network_action, "POST", b, returnedObj)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteNetwork(id int) error {
	req := NewDeleteNetworkRequest(id)
	b, _ := json.Marshal(req)
	returnedObj := &StandardResponse{}
	err := c.sendRequest(delete_network_action, "POST", b, returnedObj)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListNetworks(name string) error {
	req := NewListNetworkRequest(name)
	b, _ := json.Marshal(req)
	returnedObj := &ListNetworkResponse{}
	err := c.sendRequest(list_network_action, "GET", b, returnedObj)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RequestIP(nid, cid string) (*RequestIPResponse, error) {
	req := NewRequestIPRequest(getHostIP(), cid, nid)
	b, _ := json.Marshal(req)
	returnedObj := &RequestIPResponse{}
	err := c.sendRequest(request_ip_action, "POST", b, returnedObj)
	if err != nil {
		fmt.Printf("request ip error: %v\n", err)
		return nil, err
	}
	return returnedObj, nil
}

func (c *Client) ReleaseIP(cid, ip string) error {
	req := NewReleaseIPRequest(cid, ip)
	b, _ := json.Marshal(req)
	returnedObj := &StandardResponse{}
	err := c.sendRequest(release_ip_action, "POST", b, returnedObj)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) sendRequest(service, method string, body []byte, out ResultTrackedResponse) error {
	finalUrl := fmt.Sprintf("%s%s%s/%s", c.Url, base_path, api_version, service)
	bodyReader := bytes.NewBuffer(body)
	req, err := http.NewRequest(method, finalUrl, bodyReader)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	if err != nil {
		return fmt.Errorf("Faild to build http request")
	}

	resp, err := c.Do(req)
	if err != nil {
		return ErrPostError(service)
	}

	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode != 200 {
		return fmt.Errorf("Status code %d not 200", statusCode)
	}

	respBody, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBody, out)
	if err != nil {
		return err
	}

	if out.GetResult() != 0 {
		return ErrResultError(strconv.Itoa(out.GetResult()))
	}

	return nil
}
