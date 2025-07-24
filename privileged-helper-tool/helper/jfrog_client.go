package helper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type JfrogClient interface {
	GetArtifactsInfo() (*ArtifactsInfoResp, error)
	DownloadArtifact(artifact string) error
}

type ArtifactsInfoResp struct {
	Repo     string `json:"repo"`
	Children []struct {
		URI string `json:"uri"`
	} `json:"children"`
}

type jfrogClient struct {
	address string
	hc      *http.Client
}

func NewJfrogClient() JfrogClient {
	return &jfrogClient{
		address: "https://jfrog.wosai-inc.com/artifactory",
		hc: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func (c *jfrogClient) GetArtifactsInfo() (*ArtifactsInfoResp, error) {
	var response ArtifactsInfoResp
	url := c.address + "/api/storage/misc-local/kt-connect/latest"
	resp, err := c.hc.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get artifacts info failed with status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unexpected format, error: %s; response status code: %d", err, resp.StatusCode)
	}
	return &response, nil
}

func (c *jfrogClient) DownloadArtifact(artifact string) error {
	url := c.address + "/misc-local/kt-connect/latest/" + artifact
	resp, err := c.hc.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	file, err := os.OpenFile("/usr/local/bin/ktctl", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}
