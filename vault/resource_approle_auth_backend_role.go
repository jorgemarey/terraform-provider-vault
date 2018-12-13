package vault

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/vault/api"
	"github.com/terraform-providers/terraform-provider-vault/util"
)

var (
	approleAuthBackendRoleBackendFromPathRegex = regexp.MustCompile("^auth/(.+)/role/.+$")
	approleAuthBackendRoleNameFromPathRegex    = regexp.MustCompile("^auth/.+/role/(.+)$")
)

func approleAuthBackendRoleResource() *schema.Resource {
	return &schema.Resource{
		Create: approleAuthBackendRoleCreate,
		Read:   approleAuthBackendRoleRead,
		Update: approleAuthBackendRoleUpdate,
		Delete: approleAuthBackendRoleDelete,
		Exists: approleAuthBackendRoleExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"role_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the role.",
				ForceNew:    true,
			},
			"role_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The RoleID of the role. Autogenerated if not set.",
			},
			"bind_secret_id": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether or not to require secret_id to be present when logging in using this AppRole.",
			},
			"bound_cidr_list": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "List of CIDR blocks that can log in using the AppRole.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"policies": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Policies to be set on tokens issued using this AppRole.",
			},

			"secret_id_num_uses": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of times which a particular SecretID can be used to fetch a token from this AppRole, after which the SecretID will expire. Leaving this unset or setting it to 0 will allow unlimited uses.",
			},
			"secret_id_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of seconds a SecretID remains valid for.",
			},
			"token_num_uses": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of times issued tokens can be used. Setting this to 0 or leaving it unset means unlimited uses.",
			},
			"token_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Default number of seconds to set as the TTL for issued tokens and at renewal time.",
			},
			"token_max_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of seconds after which issued tokens can no longer be renewed.",
			},
			"period": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of seconds to set the TTL to for issued tokens upon renewal. Makes the token a periodic token, which will never expire as long as it is renewed before the TTL each period.",
			},
			"backend": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Unique name of the auth backend to configure.",
				ForceNew:    true,
				Default:     "approle",
				// standardise on no beginning or trailing slashes
				StateFunc: func(v interface{}) string {
					return strings.Trim(v.(string), "/")
				},
			},
		},
	}
}

func approleAuthBackendRoleCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	backend := d.Get("backend").(string)
	role := d.Get("role_name").(string)

	path := approleAuthBackendRolePath(backend, role)

	log.Printf("[DEBUG] Writing AppRole auth backend role %q", path)
	iPolicies := d.Get("policies").(*schema.Set).List()
	policies := make([]string, 0, len(iPolicies))
	for _, iPolicy := range iPolicies {
		policies = append(policies, iPolicy.(string))
	}

	iCIDRs := d.Get("bound_cidr_list").(*schema.Set).List()
	cidrs := make([]string, 0, len(iCIDRs))
	for _, iCIDR := range iCIDRs {
		cidrs = append(cidrs, iCIDR.(string))
	}

	data := map[string]interface{}{}
	if v, ok := d.GetOk("period"); ok {
		data["period"] = v.(int)
	}
	if len(policies) > 0 {
		data["policies"] = policies
	}
	if len(cidrs) > 0 {
		data["bound_cidr_list"] = strings.Join(cidrs, ",")
	}
	if v, ok := d.GetOkExists("bind_secret_id"); ok {
		data["bind_secret_id"] = v.(bool)
	}
	if v, ok := d.GetOk("secret_id_num_uses"); ok {
		data["secret_id_num_uses"] = v.(int)
	}
	if v, ok := d.GetOk("secret_id_ttl"); ok {
		data["secret_id_ttl"] = v.(int)
	}
	if v, ok := d.GetOk("token_num_uses"); ok {
		data["token_num_uses"] = v.(int)
	}
	if v, ok := d.GetOk("token_ttl"); ok {
		data["token_ttl"] = v.(int)
	}
	if v, ok := d.GetOk("token_max_ttl"); ok {
		data["token_max_ttl"] = v.(int)
	}

	_, err := client.Logical().Write(path, data)
	if err != nil {
		return fmt.Errorf("error writing AppRole auth backend role %q: %s", path, err)
	}
	d.SetId(path)
	log.Printf("[DEBUG] Wrote AppRole auth backend role %q", path)

	if v, ok := d.GetOk("role_id"); ok {
		log.Printf("[DEBUG] Writing AppRole auth backend role %q RoleID", path)
		_, err := client.Logical().Write(path+"/role-id", map[string]interface{}{
			"role_id": v.(string),
		})
		if err != nil {
			return fmt.Errorf("error writing AppRole auth backend role %q's RoleID: %s", path, err)
		}
		log.Printf("[DEBUG] Wrote AppRole auth backend role %q RoleID", path)
	}

	return approleAuthBackendRoleRead(d, meta)
}

func approleAuthBackendRoleRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)
	path := d.Id()

	backend, err := approleAuthBackendRoleBackendFromPath(path)
	if err != nil {
		return fmt.Errorf("invalid path %q for AppRole auth backend role: %s", path, err)
	}

	role, err := approleAuthBackendRoleNameFromPath(path)
	if err != nil {
		return fmt.Errorf("invalid path %q for AppRole auth backend role: %s", path, err)
	}

	log.Printf("[DEBUG] Reading AppRole auth backend role %q", path)
	resp, err := client.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("error reading AppRole auth backend role %q: %s", path, err)
	}
	log.Printf("[DEBUG] Read AppRole auth backend role %q", path)
	if resp == nil {
		log.Printf("[WARN] AppRole auth backend role %q not found, removing from state", path)
		d.SetId("")
		return nil
	}
	iPolicies := resp.Data["policies"].([]interface{})
	policies := make([]string, 0, len(iPolicies))
	for _, iPolicy := range iPolicies {
		policies = append(policies, iPolicy.(string))
	}

	var cidrs []string

	// NOTE: `string` is for backward-compatibility with pre-0.10.0 Vault.
	switch value := resp.Data["bound_cidr_list"].(type) {
	case string:
		if value != "" {
			cidrs = strings.Split(value, ",")
		}
	case []interface{}:
		for _, iCIDR := range value {
			cidrs = append(cidrs, iCIDR.(string))
		}
	}

	secretIDTTL, err := resp.Data["secret_id_ttl"].(json.Number).Int64()
	if err != nil {
		return fmt.Errorf("expected secret_id_ttl %q to be a number, isn't", resp.Data["secret_id_ttl"])
	}

	secretIDNumUses, err := resp.Data["secret_id_num_uses"].(json.Number).Int64()
	if err != nil {
		return fmt.Errorf("expected secret_id_num_uses %q to be a number, isn't", resp.Data["secret_id_num_uses"])
	}

	tokenTTL, err := resp.Data["token_ttl"].(json.Number).Int64()
	if err != nil {
		return fmt.Errorf("expected token_ttl %q to be a number, isn't", resp.Data["token_ttl"])
	}

	tokenNumUses, err := resp.Data["token_num_uses"].(json.Number).Int64()
	if err != nil {
		return fmt.Errorf("expected token_num_uses %q to be a number, isn't", resp.Data["token_num_uses"])
	}

	tokenMaxTTL, err := resp.Data["token_max_ttl"].(json.Number).Int64()
	if err != nil {
		return fmt.Errorf("expected token_max_ttl %q to be a number, isn't", resp.Data["token_max_ttl"])
	}

	period, err := resp.Data["period"].(json.Number).Int64()
	if err != nil {
		return fmt.Errorf("expected period %q to be a number, isn't", resp.Data["period"])
	}

	d.Set("backend", backend)
	d.Set("role_name", role)
	d.Set("period", period)
	err = d.Set("policies", policies)
	if err != nil {
		return fmt.Errorf("error setting policies in state: %s", err)
	}
	err = d.Set("bound_cidr_list", cidrs)
	if err != nil {
		return fmt.Errorf("error setting bound_cidr_list in state: %s", err)
	}
	d.Set("secret_id_num_uses", secretIDNumUses)
	d.Set("secret_id_ttl", secretIDTTL)
	d.Set("token_num_uses", tokenNumUses)
	d.Set("token_ttl", tokenTTL)
	d.Set("token_max_ttl", tokenMaxTTL)
	d.Set("period", period)
	d.Set("bind_secret_id", resp.Data["bind_secret_id"])

	log.Printf("[DEBUG] Reading AppRole auth backend role %q RoleID", path)
	resp, err = client.Logical().Read(path + "/role-id")
	if err != nil {
		return fmt.Errorf("error reading AppRole auth backend role %q RoleID: %s", path, err)
	}
	log.Printf("[DEBUG] Read AppRole auth backend role %q RoleID", path)
	if resp != nil {
		d.Set("role_id", resp.Data["role_id"])
	}

	return nil
}

func approleAuthBackendRoleUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)
	path := d.Id()

	log.Printf("[DEBUG] Updating AppRole auth backend role %q", path)
	iPolicies := d.Get("policies").(*schema.Set).List()
	policies := make([]string, 0, len(iPolicies))
	for _, iPolicy := range iPolicies {
		policies = append(policies, iPolicy.(string))
	}

	iCIDRs := d.Get("bound_cidr_list").(*schema.Set).List()
	cidrs := make([]string, 0, len(iCIDRs))
	for _, iCIDR := range iCIDRs {
		cidrs = append(cidrs, iCIDR.(string))
	}

	data := map[string]interface{}{
		"policies":           policies,
		"bound_cidr_list":    strings.Join(cidrs, ","),
		"bind_secret_id":     d.Get("bind_secret_id").(bool),
		"secret_id_num_uses": d.Get("secret_id_num_uses").(int),
		"secret_id_ttl":      d.Get("secret_id_ttl").(int),
		"token_num_uses":     d.Get("token_num_uses").(int),
		"token_ttl":          d.Get("token_ttl").(int),
		"token_max_ttl":      d.Get("token_max_ttl").(int),
		"period":             d.Get("period").(int),
	}

	_, err := client.Logical().Write(path, data)

	d.SetId(path)

	if err != nil {
		return fmt.Errorf("error updating AppRole auth backend role %q: %s", path, err)
	}
	log.Printf("[DEBUG] Updated AppRole auth backend role %q", path)

	if d.HasChange("role_id") {
		log.Printf("[DEBUG] Updating AppRole auth backend role %q RoleID", path)
		_, err := client.Logical().Write(path+"/role-id", map[string]interface{}{
			"role_id": d.Get("role_id").(string),
		})
		if err != nil {
			return fmt.Errorf("error updating AppRole auth backend role %q's RoleID: %s", path, err)
		}
		log.Printf("[DEBUG] Updated AppRole auth backend role %q RoleID", path)
	}

	return approleAuthBackendRoleRead(d, meta)

}

func approleAuthBackendRoleDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)
	path := d.Id()

	log.Printf("[DEBUG] Deleting AppRole auth backend role %q", path)
	_, err := client.Logical().Delete(path)
	if err != nil && !util.Is404(err) {
		return fmt.Errorf("error deleting AppRole auth backend role %q", path)
	} else if err != nil {
		log.Printf("[DEBUG] AppRole auth backend role %q not found, removing from state", path)
		d.SetId("")
		return nil
	}
	log.Printf("[DEBUG] Deleted AppRole auth backend role %q", path)

	return nil
}

func approleAuthBackendRoleExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*api.Client)

	path := d.Id()
	log.Printf("[DEBUG] Checking if AppRole auth backend role %q exists", path)

	resp, err := client.Logical().Read(path)
	if err != nil {
		return true, fmt.Errorf("error checking if AppRole auth backend role %q exists: %s", path, err)
	}
	log.Printf("[DEBUG] Checked if AppRole auth backend role %q exists", path)

	return resp != nil, nil
}

func approleAuthBackendRolePath(backend, role string) string {
	return "auth/" + strings.Trim(backend, "/") + "/role/" + strings.Trim(role, "/")
}

func approleAuthBackendRoleNameFromPath(path string) (string, error) {
	if !approleAuthBackendRoleNameFromPathRegex.MatchString(path) {
		return "", fmt.Errorf("no role found")
	}
	res := approleAuthBackendRoleNameFromPathRegex.FindStringSubmatch(path)
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected number of matches (%d) for role", len(res))
	}
	return res[1], nil
}

func approleAuthBackendRoleBackendFromPath(path string) (string, error) {
	if !approleAuthBackendRoleBackendFromPathRegex.MatchString(path) {
		return "", fmt.Errorf("no backend found")
	}
	res := approleAuthBackendRoleBackendFromPathRegex.FindStringSubmatch(path)
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected number of matches (%d) for backend", len(res))
	}
	return res[1], nil
}
