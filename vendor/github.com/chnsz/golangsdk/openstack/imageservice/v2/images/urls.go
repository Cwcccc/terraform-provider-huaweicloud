package images

import (
	"net/url"
	"strings"

	"github.com/chnsz/golangsdk"
	"github.com/chnsz/golangsdk/openstack/utils"
)

// `listURL` is a pure function. `listURL(c)` is a URL for which a GET
// request will respond with a list of images in the service `c`.
func listURL(c *golangsdk.ServiceClient) string {
	return c.ServiceURL("images")
}

func createURL(c *golangsdk.ServiceClient) string {
	return c.ServiceURL("images")
}

// `imageURL(c,i)` is the URL for the image identified by ID `i` in
// the service `c`.
func imageURL(c *golangsdk.ServiceClient, imageID string) string {
	return c.ServiceURL("images", imageID)
}

// `getURL(c,i)` is a URL for which a GET request will respond with
// information about the image identified by ID `i` in the service
// `c`.
func getURL(c *golangsdk.ServiceClient, imageID string) string {
	return imageURL(c, imageID)
}

func updateURL(c *golangsdk.ServiceClient, imageID string) string {
	return imageURL(c, imageID)
}

func deleteURL(c *golangsdk.ServiceClient, imageID string) string {
	return imageURL(c, imageID)
}

// builds next page full url based on current url
func nextPageURL(serviceURL, requestedNext string) (string, error) {
	base, err := utils.BaseEndpoint(serviceURL)
	if err != nil {
		return "", err
	}

	requestedNextURL, err := url.Parse(requestedNext)
	if err != nil {
		return "", err
	}

	base = golangsdk.NormalizeURL(base)
	nextPath := base + strings.TrimPrefix(requestedNextURL.Path, "/")

	nextURL, err := url.Parse(nextPath)
	if err != nil {
		return "", err
	}

	nextURL.RawQuery = requestedNextURL.RawQuery

	return nextURL.String(), nil
}