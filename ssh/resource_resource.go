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

func resourceResource() *schema.Resource {
	return &schema.Resource{
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CreateContext: resourceResourceCreate,
		ReadContext:   resourceResourceRead,
		UpdateContext: resourceResourceUpdate,
		DeleteContext: resourceResourceDelete,
		Schema: map[string]*schema.Schema{
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
			"bastion_host": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"user": {
				Type:         schema.TypeString,
				Optional:     true,
				RequiredWith: []string{"private_key"},
				ForceNew:     true,
			},
			"private_key": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				RequiredWith: []string{"user"},
			},
			"host_private_key": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				RequiredWith: []string{"user"},
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
		},
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
	privateKey := d.Get("private_key").(string)
	hostPrivateKey := d.Get("host_private_key").(string)
	host := d.Get("host").(string)
	commandsAfterFileChanges := d.Get("commands_after_file_changes").(bool)

	if len(hostPrivateKey) == 0 {
		hostPrivateKey = privateKey
	}

	// Collect SSH details
	privateIP := host
	ssh := &easyssh.MakeConfig{
		User:   user,
		Server: privateIP,
		Port:   "22",
		Key:    hostPrivateKey,
		Proxy:  http.ProxyFromEnvironment,
		Bastion: easyssh.DefaultConfig{
			User:   user,
			Server: bastionHost,
			Port:   "22",
			Key:    privateKey,
		},
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
			for i := 0; i < len(commands); i++ {
				stdout, stderr, done, err := ssh.Run(commands[i], 5*time.Minute)
				if err != nil {
					return append(diags, diag.FromErr(fmt.Errorf("command [%s]: %w", commands[i], err))...)
				} else {
					_, _ = config.Debug("command: %s\ndone: %t\nstdout:\n%s\nstderr:\n%s\n", commands[i], done, stdout, stderr)
				}
			}
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
	privateKey := d.Get("private_key").(string)
	hostPrivateKey := d.Get("host_private_key").(string)
	host := d.Get("host").(string)

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
		if privateKey == "" {
			return diag.FromErr(fmt.Errorf("privateKey must be set when 'commands' is specified"))
		}
	}
	// Collect SSH details
	privateIP := host
	ssh := &easyssh.MakeConfig{
		User:   user,
		Server: privateIP,
		Port:   "22",
		Key:    hostPrivateKey,
		Proxy:  http.ProxyFromEnvironment,
		Bastion: easyssh.DefaultConfig{
			User:   user,
			Server: bastionHost,
			Port:   "22",
			Key:    privateKey,
		},
	}

	// Provision files
	if err := copyFiles(ssh, config, createFiles); err != nil {
		return diag.FromErr(fmt.Errorf("copying files to remote: %w", err))
	}

	// Run commands
	for i := 0; i < len(commands); i++ {
		stdout, stderr, done, err := ssh.Run(commands[i], 5*time.Minute)
		if err != nil {
			return append(diags, diag.FromErr(fmt.Errorf("command [%s]: %w", commands[i], err))...)
		} else {
			_, _ = config.Debug("command: %s\ndone: %t\nstdout:\n%s\nstderr:\n%s\n", commands[i], done, stdout, stderr)
		}
	}

	d.SetId(fmt.Sprintf("%d", rand.Int()))
	return diags
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
			// Should we fail the complete provision on errors here?
			_ = ssh.WriteFile(buffer, int64(buffer.Len()), f.Destination)
			_, _ = config.Debug("Created remote file %s:%s: %d bytes\n", ssh.Server, f.Destination, len(f.Content))
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
