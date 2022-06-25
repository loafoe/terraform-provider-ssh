package ssh

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/loafoe/easyssh-proxy/v2"
)

var resourceSchema map[string]*schema.Schema = map[string]*schema.Schema{
	"triggers": {
		Description: "A map of arbitrary strings that, when changed, will force the 'hsdp_container_host_exec' resource to be replaced, re-running any associated commands.",
		Type:        schema.TypeMap,
		Optional:    true,
		ForceNew:    true,
	},
	"host": {
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
	},
	"port": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "22",
	},
	"bastion_host": {
		Type:     schema.TypeString,
		Optional: true,
	},
	"bastion_port": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "22",
	},
	"user": {
		Type:     schema.TypeString,
		Optional: true,
		ForceNew: true,
	},
	"host_user": {
		Type:     schema.TypeString,
		Optional: true,
		ForceNew: true,
	},
	"private_key": {
		Type:      schema.TypeString,
		Optional:  true,
		Sensitive: true,
	},
	"host_private_key": {
		Type:      schema.TypeString,
		Optional:  true,
		Sensitive: true,
	},
	"agent": {
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
	},
	"commands": {
		Type:     schema.TypeList,
		MaxItems: 100,
		Optional: true,
		Elem:     &schema.Schema{Type: schema.TypeString},
		ForceNew: true,
	},
	"commands_after_file_changes": {
		Type:     schema.TypeBool,
		Optional: true,
		Default:  true,
	},
	"timeout": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "5m",
	},
	"result": {
		Type:     schema.TypeString,
		Computed: true,
	},
	"file": {
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"source": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"content": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"destination": {
					Type:     schema.TypeString,
					Required: true,
				},
				"permissions": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"owner": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"group": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	},
}

func resourceResource() *schema.Resource {
	return &schema.Resource{
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CreateContext: resourceResourceCreate,
		ReadContext:   resourceResourceRead,
		UpdateContext: resourceResourceUpdate,
		DeleteContext: resourceResourceDelete,
		Schema:        resourceSchema,
	}
}

func resourceResourceDelete(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	d.SetId("")
	return diags
}

func resourceResourceUpdate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	config := m.(*Config)

	bastionHost := d.Get("bastion_host").(string)
	user := d.Get("user").(string)
	hostUser := d.Get("host_user").(string)
	privateKey := d.Get("private_key").(string)
	hostPrivateKey := d.Get("host_private_key").(string)
	host := d.Get("host").(string)
	commandsAfterFileChanges := d.Get("commands_after_file_changes").(bool)
	agent := d.Get("agent").(bool)
	timeout := d.Get("timeout").(string)
	port := d.Get("port").(string)
	bastionPort := d.Get("bastion_port").(string)

	timeoutValue, err := calcTimeout(timeout)
	if err != nil {
		return diag.FromErr(err)
	}

	if len(hostUser) == 0 {
		hostUser = user
	}

	if len(hostPrivateKey) == 0 {
		hostPrivateKey = privateKey
	}

	// Collect SSH details
	privateIP := host
	ssh := &easyssh.MakeConfig{
		User:   hostUser,
		Server: privateIP,
		Port:   port,
		Proxy:  http.ProxyFromEnvironment,
		Bastion: easyssh.DefaultConfig{
			User:   user,
			Server: bastionHost,
			Port:   bastionPort,
		},
	}
	if hostPrivateKey != "" {
		if agent {
			return diag.FromErr(fmt.Errorf("agent mode is enabled, not expecting a private key"))
		}
		ssh.Key = hostPrivateKey
	}
	if privateKey != "" {
		ssh.Bastion.Key = privateKey
	}

	if d.HasChange("file") {
		createFiles, diags := collectFilesToCreate(d)
		if len(diags) > 0 {
			return diags
		}
		if err := copyFiles(ssh, config, createFiles); err != nil {
			return diag.FromErr(fmt.Errorf("copying files to remote: %w", err))
		}
		if commandsAfterFileChanges {
			commands, diags := collectCommands(d)
			if len(diags) > 0 {
				return diags
			}
			// Run commands
			stdout, errDiags, err := runCommands(commands, time.Duration(timeoutValue), ssh, m)
			if err != nil {
				return errDiags
			}
			_ = d.Set("result", stdout)
		}
	}
	return diags
}

func resourceResourceRead(_ context.Context, _ *schema.ResourceData, _ interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

func resourceResourceCreate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	config := m.(*Config)

	bastionHost := d.Get("bastion_host").(string)
	user := d.Get("user").(string)
	hostUser := d.Get("host_user").(string)
	privateKey := d.Get("private_key").(string)
	hostPrivateKey := d.Get("host_private_key").(string)
	host := d.Get("host").(string)
	agent := d.Get("agent").(bool)
	timeout := d.Get("timeout").(string)
	port := d.Get("port").(string)
	bastionPort := d.Get("bastion_port").(string)

	timeoutValue, err := calcTimeout(timeout)
	if err != nil {
		return diag.FromErr(err)
	}

	if len(hostUser) == 0 {
		hostUser = user
	}

	if len(hostPrivateKey) == 0 {
		hostPrivateKey = privateKey
	}

	// Fetch files first before starting provisioning
	createFiles, diags := collectFilesToCreate(d)
	if len(diags) > 0 {
		return diags
	}
	// And commands
	commands, diags := collectCommands(d)
	if len(diags) > 0 {
		return diags
	}
	if len(commands) > 0 {
		if user == "" {
			return diag.FromErr(fmt.Errorf("user must be set when 'commands' is specified"))
		}
		if !agent && privateKey == "" {
			return diag.FromErr(fmt.Errorf("privateKey must be set when 'commands' is specified and 'agent' is false"))
		}
	}
	// Collect SSH details
	privateIP := host
	ssh := &easyssh.MakeConfig{
		User:   hostUser,
		Server: privateIP,
		Port:   port,
		Proxy:  http.ProxyFromEnvironment,
		Bastion: easyssh.DefaultConfig{
			User:   user,
			Server: bastionHost,
			Port:   bastionPort,
		},
	}
	if hostPrivateKey != "" {
		if agent {
			return diag.FromErr(fmt.Errorf("agent mode is enabled, not expecting a private key"))
		}
		ssh.Key = hostPrivateKey
	}
	if privateKey != "" {
		ssh.Bastion.Key = privateKey
	}

	// Provision files
	if err := copyFiles(ssh, config, createFiles); err != nil {
		return diag.FromErr(fmt.Errorf("copying files to remote: %w", err))
	}

	// Run commands
	stdout, errDiags, err := runCommands(commands, time.Duration(timeoutValue), ssh, m)
	if err != nil {
		return errDiags
	}

	_ = d.Set("result", stdout)
	d.SetId(fmt.Sprintf("%d", rand.Int()))
	return diags
}

func runCommands(commands []string, timeout time.Duration, ssh *easyssh.MakeConfig, m interface{}) (string, diag.Diagnostics, error) {
	var diags diag.Diagnostics
	var stdout, stderr string
	var done bool
	var err error
	config := m.(*Config)

	for i := 0; i < len(commands); i++ {
		stdout, stderr, done, err = ssh.Run(commands[i], timeout*time.Second)
		_, _ = config.Debug("command: %s\ndone: %t\nstdout:\n%s\nstderr:\n%s\n", commands[i], done, stdout, stderr)
		if err != nil {
			_, _ = config.Debug("error: %v\n", err)
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  fmt.Sprintf("execution of command '%s' failed. stdout output", commands[i]),
				Detail:   stdout,
			})
			if stderr != "" {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  "stderr output",
					Detail:   stderr,
				})
			}
			return stdout, diags, err
		}
	}
	return stdout, diags, nil
}

func copyFiles(ssh *easyssh.MakeConfig, config *Config, createFiles []provisionFile) error {
	for _, f := range createFiles {
		if f.Source != "" {
			src, srcErr := os.Open(f.Source)
			if srcErr != nil {
				_, _ = config.Debug("Failed to open source file %s: %v\n", f.Source, srcErr)
				return srcErr
			}
			srcStat, statErr := src.Stat()
			if statErr != nil {
				_, _ = config.Debug("Failed to stat source file %s: %v\n", f.Source, statErr)
				_ = src.Close()
				return statErr
			}
			_ = ssh.WriteFile(src, srcStat.Size(), f.Destination)
			_, _ = config.Debug("Copied %s to remote file %s:%s: %d bytes\n", f.Source, ssh.Server, f.Destination, srcStat.Size())
			_ = src.Close()
		} else {
			buffer := bytes.NewBufferString(f.Content)
			if err := ssh.WriteFile(buffer, int64(buffer.Len()), f.Destination); err != nil {
				_, _ = config.Debug("Failed to copy content to remote file %s:%s:%s: %v\n", ssh.Server, ssh.Port, f.Destination, err)
				return err
			}
			_, _ = config.Debug("Created remote file %s:%s:%s: %d bytes\n", ssh.Server, ssh.Port, f.Destination, len(f.Content))
		}
		// Permissions change
		if f.Permissions != "" {
			outStr, errStr, _, err := ssh.Run(fmt.Sprintf("chmod %s \"%s\"", f.Permissions, f.Destination))
			_, _ = config.Debug("Permissions file %s:%s: %v %v\n", f.Destination, f.Permissions, outStr, errStr)
			if err != nil {
				return err
			}
		}
		// Owner
		if f.Owner != "" {
			outStr, errStr, _, err := ssh.Run(fmt.Sprintf("chown %s \"%s\"", f.Owner, f.Destination))
			_, _ = config.Debug("Owner file %s:%s: %v %v\n", f.Destination, f.Owner, outStr, errStr)
			if err != nil {
				return err
			}
		}
		// Group
		if f.Group != "" {
			outStr, errStr, _, err := ssh.Run(fmt.Sprintf("chgrp %s \"%s\"", f.Group, f.Destination))
			_, _ = config.Debug("Group file %s:%s: %v %v\n", f.Destination, f.Group, outStr, errStr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func collectCommands(d *schema.ResourceData) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	list := d.Get("commands").([]interface{})
	commands := make([]string, 0)
	for i := 0; i < len(list); i++ {
		commands = append(commands, list[i].(string))
	}
	return commands, diags

}

type provisionFile struct {
	Source      string
	Content     string
	Destination string
	Permissions string
	Owner       string
	Group       string
}

func collectFilesToCreate(d *schema.ResourceData) ([]provisionFile, diag.Diagnostics) {
	var diags diag.Diagnostics
	files := make([]provisionFile, 0)
	if v, ok := d.GetOk("file"); ok {
		vL := v.(*schema.Set).List()
		for _, vi := range vL {
			mVi := vi.(map[string]interface{})
			file := provisionFile{
				Source:      mVi["source"].(string),
				Content:     mVi["content"].(string),
				Destination: mVi["destination"].(string),
				Permissions: mVi["permissions"].(string),
				Owner:       mVi["owner"].(string),
				Group:       mVi["group"].(string),
			}
			if file.Source == "" && file.Content == "" {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  "conflict in file block",
					Detail:   fmt.Sprintf("file %s has neither 'source' or 'content', set one", file.Destination),
				})
				continue
			}
			if file.Source != "" && file.Content != "" {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  "conflict in file block",
					Detail:   fmt.Sprintf("file %s has conflicting 'source' and 'content', choose only one", file.Destination),
				})
				continue
			}
			if file.Source != "" {
				src, srcErr := os.Open(file.Source)
				if srcErr != nil {
					diags = append(diags, diag.Diagnostic{
						Severity: diag.Error,
						Summary:  "issue with source",
						Detail:   fmt.Sprintf("file %s: %v", file.Source, srcErr),
					})
					continue
				}
				_, statErr := src.Stat()
				if statErr != nil {
					diags = append(diags, diag.Diagnostic{
						Severity: diag.Error,
						Summary:  "issue with source stat",
						Detail:   fmt.Sprintf("file %s: %v", file.Source, statErr),
					})
					_ = src.Close()
					continue
				}
				_ = src.Close()
			}
			files = append(files, file)
		}
	}
	return files, diags
}

func calcTimeout(timeout string) (int, error) {
	var unit string
	var value int
	scanned, err := fmt.Sscanf(timeout, "%d%s", &value, &unit)
	if err != nil {
		return 0, fmt.Errorf("calcTimeout scan [%s]: %w", timeout, err)
	}
	if scanned != 2 {
		return 0, fmt.Errorf("invalid timeout format: %s", timeout)
	}
	seconds := 0
	switch unit {
	case "s":
		seconds = value
	case "m":
		seconds = 60 * value
	case "h":
		seconds = 3600 * value
	case "d":
		seconds = 86400 * value
	default:
		return 0, fmt.Errorf("unit '%s' not supported", unit)
	}
	if seconds < 60 {
		return 0, fmt.Errorf("a value less than 60 seconds is not supported")
	}
	return seconds, nil
}
