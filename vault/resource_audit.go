package vault

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/vault/api"
)

func auditResource() *schema.Resource {
	return &schema.Resource{
		Create: auditWrite,
		Delete: auditDelete,
		Read:   auditRead,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Specifies the path in which to enable the audit device",
			},

			"type": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Specifies the type of the audit device",
			},

			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Required:    false,
				ForceNew:    true,
				Description: "Specifies a human-friendly description of the audit device",
			},

			"options": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Description: "Specifies configuration options to pass to the audit device itself. This is dependent on the audit device type",
			},

			"local": {
				Type:        schema.TypeBool,
				Optional:    true,
				Required:    false,
				Default:     false,
				ForceNew:    true,
				Description: "Specifies if the audit device is a local only",
			},
		},
	}
}

func auditWrite(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	path := d.Get("path").(string)

	log.Printf("[DEBUG] Creating audit %s in Vault", path)

	options := map[string]string{}
	if v, ok := d.GetOk("options"); ok {
		optionsI, ok := v.(map[string]interface{})
		if !ok {
			return fmt.Errorf("error: options should be a map")
		}
		for k, v := range optionsI {
			if vs, ok := v.(string); ok {
				options[k] = vs
			} else {
				return fmt.Errorf("error: options should be a string -> string map")
			}
		}
	}

	if err := client.Sys().EnableAudit(
		path,
		d.Get("type").(string),
		d.Get("description").(string),
		options,
	); err != nil {
		return fmt.Errorf("error writing to Vault: %s", err)
	}

	d.SetId(path)

	return nil
}

func auditDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	path := d.Id()

	log.Printf("[DEBUG] Removing audit %s from Vault", path)

	if err := client.Sys().DisableAudit(path); err != nil {
		return fmt.Errorf("error deleting from Vault: %s", err)
	}

	return nil
}

func auditRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	path := d.Id()

	log.Printf("[DEBUG] Reading audit %s from Vault", path)

	audits, err := client.Sys().ListAudit()
	if err != nil {
		return fmt.Errorf("error reading from Vault: %s", err)
	}

	// path can have a trailing slash, but doesn't need to have one
	// this standardises on having a trailing slash, which is how the
	// API always responds.
	audit, ok := audits[strings.Trim(path, "/")+"/"]
	if !ok {
		log.Printf("[WARN] Audit %q not found, removing from state.", path)
		d.SetId("")
		return nil
	}

	d.Set("path", path)
	d.Set("type", audit.Type)
	d.Set("description", audit.Description)
	d.Set("local", audit.Local)
	if audit.Options != nil { // This can be nil
		d.Set("options", audit.Options)
	}

	return nil
}
