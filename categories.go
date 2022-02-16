package bigcommerce

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

// Category is a BC category object
type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ParentID  int64  `json:"parent_id"`
	Visible   bool   `json:"is_visible"`
	FullName  string `json:"-"`
	CustomURL struct {
		URL        string `json:"url"`
		Customized bool   `json:"is_customized"`
	} `json:"custom_url"`
	URL string `json:"-"`
}

// GetAllCategories returns a list of categories, handling pagination
// context: the BigCommerce context (e.g. stores/23412341234) where 23412341234 is the store hash
// xAuthToken: the BigCommerce Store's X-Auth-Token coming from store credentials (see AuthContext)
func (bc *BigCommerce) GetAllCategories(context, xAuthToken string) ([]Category, error) {
	cs := []Category{}
	var csp []Category
	page := 1
	more := true
	var err error
	retries := 0
	for more {
		csp, more, err = bc.GetCategories(context, xAuthToken, page)
		if err != nil {
			log.Println(err)
			retries++
			if retries > bc.MaxRetries {
				return cs, fmt.Errorf("max retries reached")
			}
			break
		}
		cs = append(cs, csp...)
		page++
	}
	extidmap := map[int64]int{}
	for i, c := range cs {
		extidmap[c.ID] = i
	}
	for i := range cs {
		cs[i].URL = cs[i].CustomURL.URL
		// get A > B > C fancy name
		cs[i].FullName = bc.getFullCategoryName(cs, i, extidmap)
	}
	return cs, err
}

// GetCategories returns a list of categories, handling pagination
// context: the BigCommerce context (e.g. stores/23412341234) where 23412341234 is the store hash
// xAuthToken: the BigCommerce Store's X-Auth-Token coming from store credentials (see AuthContext)
// page: the page number to download
func (bc *BigCommerce) GetCategories(context, xAuthToken string, page int) ([]Category, bool, error) {
	url := context + "/v3/catalog/categories?include_fields=name,parent_id,is_visible,custom_url&page=" + strconv.Itoa(page)

	req := bc.getAPIRequest(http.MethodGet, url, xAuthToken, nil)
	res, err := bc.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}

	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return nil, false, ErrNoContent
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, false, err
	}

	var pp struct {
		Data []Category `json:"data"`
		Meta struct {
			Pagination struct {
				Total       int64       `json:"total"`
				Count       int64       `json:"count"`
				PerPage     int64       `json:"per_page"`
				CurrentPage int64       `json:"current_page"`
				TotalPages  int64       `json:"total_pages"`
				Links       interface{} `json:"links"`
				TooMany     bool        `json:"too_many"`
			} `json:"pagination"`
		} `json:"meta"`
	}
	err = json.Unmarshal(body, &pp)
	if err != nil {
		return nil, false, err
	}
	return pp.Data, pp.Meta.Pagination.CurrentPage < pp.Meta.Pagination.TotalPages, nil
}

func (bc *BigCommerce) getFullCategoryName(cs []Category, i int, extidmap map[int64]int) string {
	if cs[i].ParentID == 0 {
		return cs[i].Name
	}
	return bc.getFullCategoryName(cs, extidmap[cs[i].ParentID], extidmap) + " > " + cs[i].Name
}
