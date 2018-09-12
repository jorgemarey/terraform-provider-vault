---
layout: "vault"
page_title: "Vault: vault_audit resource"
sidebar_current: "docs-vault-resource-audit"
description: |-
  Managing the audit backends in Vault
---

# vault\_audit


## Example Usage

```hcl
resource "vault_audit" "syslog" {
    path = "syslog"
    type = "syslog"
    options {
        facility = "LOCAL0"
        tag = "vault_audit"
    }
}
```

## Argument Reference

The following arguments are supported:

* `path` - (Required) Specifies the path in which to enable the audit device

* `type` - (Required) Specifies the type of the audit device

* `description` - (Optional) Specifies a human-friendly description of the audit device

* `options` - (Optional) Specifies configuration options to pass to the audit device itself. This is dependent on the audit device type

* `local` - (Optional) Specifies if the audit device is a local only
