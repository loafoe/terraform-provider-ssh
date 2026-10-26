package ssh

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/loafoe/easyssh-proxy/v2"
)

func resourceResource() *schema.Resource {
	return &schema.Resource{
		SchemaVersion: 5,
		CreateContext: resourceResourceCreate,
		ReadContext:   resourceResourceRead,
		UpdateContext: resourceResourceUpdate,
		DeleteContext: resourceResourceDelete,
		CustomizeDiff: customDiff,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		StateUpgraders: []schema.StateUpgrader{
			{
				Type:    resourceResourceV0().CoreConfigSchema().ImpliedType(),
				Upgrade: patchResourceV0,
				Version: 0,
			},
			{
				Type:    resourceResourceV1().CoreConfigSchema().ImpliedType(),
				Upgrade: patchResourceV1,
				Version: 1,
			},
			{
				Type:    resourceResourceV2().CoreConfigSchema().ImpliedType(),
				Upgrade: patchResourceV2,
				Version: 2,
			},
			{
				Type:    resourceResourceV3().CoreConfigSchema().ImpliedType(),
				Upgrade: patchResourceV3,
				Version: 3,
			},
			{
				Type:    resourceResourceV4().CoreConfigSchema().ImpliedType(),
				Upgrade: patchResourceV4,
				Version: 4,
			},
		},
		Schema: sshResourceSchema(false),
	}
}

func customDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	triggersChanged := d.HasChange("triggers")
	if !triggersChanged {
		configured, configuredKnown := triggerCount(d.GetRawConfig())
		prior, priorKnown := triggerCount(d.GetRawState())
		triggersChanged = configuredKnown && priorKnown && configured == 0 && prior > 0
	}
	if d.HasChange("file") || d.HasChange("commands") || triggersChanged {
		if err := d.SetNewComputed("result"); err != nil {
			return err
		}
	}
	if triggersChanged {
		// A trigger can be derived from an attribute that another provider only
		// finalizes during apply. Marking changed triggers as unknown lets the
		// applied value be recorded without Terraform reporting an inconsistent
		// final plan. ForceNew still guarantees the resource is replaced.
		if err := d.SetNewComputed("triggers"); err != nil {
			return err
		}
	}
	return nil
}

// triggerCount returns the number of configured trigger entries. The bool is
// false when the value is not available or not yet known.
func triggerCount(raw cty.Value) (int, bool) {
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() || !raw.Type().HasAttribute("triggers") {
		return 0, false
	}

	triggers := raw.GetAttr("triggers")
	if triggers.IsNull() {
		return 0, true
	}
	if !triggers.IsKnown() || !triggers.CanIterateElements() {
		return 0, false
	}
	return triggers.LengthInt(), true
}

func sshResourceSchema(sensitive bool) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"when": {
			Description:  "Determines when the commands is to be executed. Options are 'create' or 'destroy'",
			Type:         schema.TypeString,
			Optional:     true,
			Default:      "create",
			ValidateFunc: validation.StringInSlice([]string{"create", "destroy"}, false),
		},
		"triggers": {
			Description: "A map of arbitrary strings that, when changed, will force the 'hsdp_container_host_exec' resource to be replaced, re-running any associated commands.",
			Type:        schema.TypeMap,
			Optional:    true,
			Computed:    true,
			ForceNew:    true,
		},
		"host": {
			Description:  "The IP address or DNS hostname of the target server, without a port.",
			Type:         schema.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validateHost,
		},
		"port": {
			Type:     schema.TypeString,
			Optional: true,
			Default:  "22",
		},
		"bastion_host": {
			Description:  "The IP address or DNS hostname of the bastion host, without a port.",
			Type:         schema.TypeString,
			Optional:     true,
			ValidateFunc: validateOptionalHost,
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
			Type:       schema.TypeString,
			Optional:   true,
			ForceNew:   true,
			Deprecated: "Use 'user' and 'bastion_user'",
		},
		"bastion_user": {
			Type:     schema.TypeString,
			Optional: true,
			ForceNew: true,
		},
		"password": {
			Type:      schema.TypeString,
			Optional:  true,
			Sensitive: true,
		},
		"bastion_password": {
			Type:      schema.TypeString,
			Optional:  true,
			Sensitive: true,
		},
		"private_key": {
			Type:      schema.TypeString,
			Optional:  true,
			Sensitive: true,
		},
		"host_private_key": {
			Type:       schema.TypeString,
			Optional:   true,
			Sensitive:  true,
			Deprecated: "Use 'private_key' and 'bastion_private_key'",
		},
		"bastion_private_key": {
			Type:      schema.TypeString,
			Optional:  true,
			Sensitive: true,
		},
		"agent": {
			Type:     schema.TypeBool,
			Optional: true,
			Default:  false,
		},
		"pre_commands": {
			Type:     schema.TypeList,
			MaxItems: 100,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"commands": {
			Type:     schema.TypeList,
			MaxItems: 100,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
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
		"ignore_no_supported_methods_remain": {
			Type:     schema.TypeBool,
			Optional: true,
			Default:  false,
		},
		"retry_delay": {
			Type:     schema.TypeString,
			Optional: true,
			Default:  "10s",
		},
		"result": {
			Type:      schema.TypeString,
			Computed:  true,
			Sensitive: sensitive,
		},
		"file": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"source": {
						Description: "Path to a local file to copy to the remote host. Conflicts with 'content'.",
						Type:        schema.TypeString,
						Optional:    true,
					},
					"content": {
						Description: "Content to write to the remote file. Conflicts with 'source'. An empty string creates an empty file.",
						Type:        schema.TypeString,
						Optional:    true,
						Sensitive:   sensitive,
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
}

func resourceResourceDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	when := d.Get("when").(string)

	if when == "destroy" {
		diags = mainRun(ctx, d, m, false)
	}
	if !hasErrors(diags) {
		d.SetId("")
	}
	return diags
}

func hasErrors(diags diag.Diagnostics) bool {
	for _, d := range diags {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
}

func validateResource(d *schema.ResourceData) diag.Diagnostics {
	var diags diag.Diagnostics
	var commands []string

	timeout := d.Get("timeout").(string)
	user := d.Get("user").(string)
	agent := d.Get("agent").(bool)
	privateKey := d.Get("private_key").(string)
	hostPrivateKey := d.Get("host_private_key").(string)
	retryDelay := d.Get("retry_delay").(string)
	password := d.Get("password").(string)

	timeoutValue, err := time.ParseDuration(timeout)
	if err != nil {
		return diag.FromErr(fmt.Errorf("timeout value: %w", err))
	}
	retryDelayValue, err := time.ParseDuration(retryDelay)
	if err != nil {
		return diag.FromErr(fmt.Errorf("retry_delay value: %w", err))
	}
	if retryDelayValue >= timeoutValue {
		return diag.FromErr(fmt.Errorf("retry_delay cannot be greater than timeout (%d >= %d)", retryDelayValue, timeoutValue))
	}

	if len(hostPrivateKey) == 0 {
		hostPrivateKey = privateKey
	}
	_, diags = collectFilesToCreate(d)
	if len(diags) > 0 {
		return diags
	}
	commands, diags = collectCommands(d, "commands")
	if len(diags) > 0 {
		return diags
	}
	if len(commands) > 0 {
		if user == "" {
			return diag.FromErr(fmt.Errorf("user must be set when 'commands' is specified"))
		}
		if !agent && privateKey == "" && password == "" {
			return diag.FromErr(fmt.Errorf("'privateKey' must be set when 'commands' is specified and 'agent' is false and no 'password' is given"))
		}
	}
	if agent && (hostPrivateKey != "" || privateKey != "" || password != "") {
		return diag.FromErr(fmt.Errorf("agent mode is enabled, not expecting a password or private key"))
	}
	return diags
}

func mainRun(_ context.Context, d *schema.ResourceData, m interface{}, onUpdate bool) diag.Diagnostics {
	config := m.(*Config)

	if diags := validateResource(d); len(diags) > 0 {
		return diags
	}

	bastionHost := d.Get("bastion_host").(string)
	user := d.Get("user").(string)
	hostUser := d.Get("host_user").(string)
	bastionUser := d.Get("bastion_user").(string)
	password := d.Get("password").(string)
	bastionPassword := d.Get("bastion_password").(string)
	privateKey := d.Get("private_key").(string)
	privateKeyPassphrase, _ := schema.EnvDefaultFunc("SSH_PRIVATE_KEY_PASSPHRASE", "")()
	hostPrivateKey := d.Get("host_private_key").(string)
	bastionPrivateKey := d.Get("bastion_private_key").(string)
	bastionPrivateKeyPassphrase, _ := schema.EnvDefaultFunc("SSH_BASTION_PRIVATE_KEY_PASSPHRASE", "")()
	host := d.Get("host").(string)
	timeout := d.Get("timeout").(string)
	retryDelay := d.Get("retry_delay").(string)
	port := d.Get("port").(string)
	bastionPort := d.Get("bastion_port").(string)
	commandsAfterFileChanges := d.Get("commands_after_file_changes").(bool)

	var sshRetryConfig SSHRetryConfig

	sshRetryConfig.timeout, _ = time.ParseDuration(timeout)
	sshRetryConfig.retryDelay, _ = time.ParseDuration(retryDelay)
	sshRetryConfig.ignoreUnsupportedAuthMethods = d.Get("ignore_no_supported_methods_remain").(bool)

	if len(hostUser) == 0 {
		hostUser = user
	}
	if len(hostPrivateKey) == 0 {
		hostPrivateKey = privateKey
	}
	// Pre commands
	preCommands, diags := collectCommands(d, "pre_commands")
	if len(diags) > 0 {
		return diags
	}
	// Fetch files first before starting provisioning
	createFiles, diags := collectFilesToCreate(d)
	if len(diags) > 0 {
		return diags
	}
	// And commands
	commands, diags := collectCommands(d, "commands")
	if len(diags) > 0 {
		return diags
	}

	// Collect SSH details
	privateIP := host
	ssh := &easyssh.MakeConfig{
		User:       hostUser,
		Server:     privateIP,
		Port:       port,
		Key:        privateKey,
		Passphrase: privateKeyPassphrase.(string),
		Proxy:      proxyFromEnvironment,
		Bastion: easyssh.DefaultConfig{
			User:       user,
			Server:     bastionHost,
			Passphrase: bastionPrivateKeyPassphrase.(string),
			Port:       bastionPort,
		},
	}
	if password != "" {
		ssh.Password = password
	}
	if bastionPassword != "" {
		ssh.Bastion.Password = bastionPassword
	}
	if bastionUser != "" {
		ssh.Bastion.User = bastionUser
	}
	if hostPrivateKey != "" {
		ssh.Key = hostPrivateKey
	}
	if privateKey != "" {
		ssh.Bastion.Key = privateKey
	}
	if bastionPrivateKey != "" {
		ssh.Bastion.Key = bastionPrivateKey
	}

	if onUpdate && !(d.HasChange("file") || d.HasChange("commands")) {
		return diags
	}

	ctx, cancel := context.WithTimeout(context.Background(), sshRetryConfig.timeout)
	defer cancel()

	// Run pre commands
	if len(preCommands) > 0 {
		_, errDiags, err := runCommands(ctx, preCommands, ssh, sshRetryConfig, m)
		if err != nil {
			return errDiags
		}
	}
	// Provision files
	if err := copyFiles(ctx, sshRetryConfig.retryDelay, ssh, config, createFiles); err != nil {
		return diag.FromErr(fmt.Errorf("copying files to remote: %w", ctx.Err()))
	}

	if onUpdate && !commandsAfterFileChanges {
		return diags
	}

	// Run commands
	stdout, errDiags, err := runCommands(ctx, commands, ssh, sshRetryConfig, m)
	if err != nil {
		return errDiags
	}

	_ = d.Set("result", stdout)

	return diags
}

func resourceResourceUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	when := d.Get("when").(string)

	if when == "create" {
		diags = mainRun(ctx, d, m, true)
	}
	if !hasErrors(diags) {
		if err := setTriggersFromConfig(d); err != nil {
			return diag.FromErr(err)
		}
	}
	return diags
}

func resourceResourceRead(_ context.Context, _ *schema.ResourceData, _ interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

func resourceResourceCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	when := d.Get("when").(string)

	if when == "create" {
		diags = mainRun(ctx, d, m, false)
	} else {
		diags = validateResource(d)
	}

	if !hasErrors(diags) {
		if err := setTriggersFromConfig(d); err != nil {
			return diag.FromErr(err)
		}
		d.SetId(fmt.Sprintf("%d", rand.Int()))
	}
	return diags
}

type SSHRetryConfig struct {
	retryDelay                   time.Duration
	timeout                      time.Duration
	ignoreUnsupportedAuthMethods bool
}

func runCommands(ctx context.Context, commands []string, ssh *easyssh.MakeConfig, sshRetryConfig SSHRetryConfig, m interface{}) (string, diag.Diagnostics, error) {
	var diags diag.Diagnostics
	var stdout, stderr string
	var done bool
	var err error
	config := m.(*Config)

	for i := 0; i < len(commands); i++ {
		for {
			stdout, stderr, done, err = ssh.Run(commands[i], sshRetryConfig.timeout)
			_, _ = config.Debug("command: %s\ndone: %t\nstdout:\n%s\nstderr:\n%s\nerror: %v\n", commands[i], done, stdout, stderr, err)
			if err == nil {
				break
			}
			if !sshRetryConfig.ignoreUnsupportedAuthMethods && strings.Contains(err.Error(), "no supported methods remain") {
				diags = append(diags, diag.FromErr(err)...)
				return stdout, diags, err
			}

			select {
			case <-time.After(sshRetryConfig.retryDelay):
				// Retry

			case <-ctx.Done():
				_, _ = config.Debug("error: %v\n", err)
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  fmt.Sprintf("execution of command '%s' failed: %s: %s", commands[i], ctx.Err(), err),
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
	}
	return stdout, diags, nil
}

func copyFiles(ctx context.Context, retryDelay time.Duration, ssh *easyssh.MakeConfig, config *Config, createFiles []provisionFile) error {
	for _, f := range createFiles {
		copyFile := func(f provisionFile) error {
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
			return nil
		}
		for {
			err := copyFile(f)
			if err == nil {
				break
			}
			select {
			case <-time.After(retryDelay):
			// Retry
			case <-ctx.Done():
				return fmt.Errorf("%s: %w", ctx.Err(), err)
			}
		}
	}
	return nil
}

func collectCommands(d *schema.ResourceData, field string) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	list := d.Get(field).([]interface{})
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

// provisionFilePresence tracks which of the mutually exclusive source/content
// fields were explicitly set in the configuration. ResourceData returns the
// zero value for both fields, so this is needed to distinguish an omitted
// content field from an explicitly empty one.
type provisionFilePresence struct {
	Source  bool
	Content bool
}

func configuredFiles(rawConfig cty.Value) map[string]provisionFilePresence {
	presence := make(map[string]provisionFilePresence)
	if rawConfig.IsNull() || !rawConfig.IsKnown() || !rawConfig.Type().IsObjectType() || !rawConfig.Type().HasAttribute("file") {
		return presence
	}

	files := rawConfig.GetAttr("file")
	if files.IsNull() || !files.IsKnown() || !files.CanIterateElements() {
		return presence
	}

	for it := files.ElementIterator(); it.Next(); {
		_, value := it.Element()
		if value.IsNull() || !value.IsKnown() || !value.Type().IsObjectType() {
			continue
		}

		destination := value.GetAttr("destination")
		if destination.IsNull() || !destination.IsKnown() {
			continue
		}

		p := presence[destination.AsString()]
		p.Source = p.Source || !value.GetAttr("source").IsNull()
		p.Content = p.Content || !value.GetAttr("content").IsNull()
		presence[destination.AsString()] = p
	}

	return presence
}

func triggersFromRawConfig(rawConfig cty.Value) (map[string]interface{}, bool) {
	if rawConfig.IsNull() || !rawConfig.IsKnown() || !rawConfig.Type().IsObjectType() || !rawConfig.Type().HasAttribute("triggers") {
		return nil, false
	}

	triggers := rawConfig.GetAttr("triggers")
	if triggers.IsNull() {
		return map[string]interface{}{}, true
	}
	if !triggers.IsKnown() || !triggers.CanIterateElements() {
		return nil, false
	}

	values := make(map[string]interface{}, triggers.LengthInt())
	for it := triggers.ElementIterator(); it.Next(); {
		key, value := it.Element()
		if key.IsNull() || !key.IsKnown() || value.IsNull() || !value.IsKnown() {
			continue
		}
		values[key.AsString()] = value.AsString()
	}
	return values, true
}

func setTriggersFromConfig(d *schema.ResourceData) error {
	triggers, ok := triggersFromRawConfig(d.GetRawConfig())
	if !ok {
		return nil
	}
	return d.Set("triggers", triggers)
}

func configuredFilesForResource(d *schema.ResourceData) map[string]provisionFilePresence {
	for _, rawConfig := range []cty.Value{d.GetRawConfig(), d.GetRawPlan(), d.GetRawState()} {
		if presence := configuredFiles(rawConfig); len(presence) > 0 {
			return presence
		}
	}
	return map[string]provisionFilePresence{}
}

func validateProvisionFile(file provisionFile, presence provisionFilePresence) *diag.Diagnostic {
	sourceSet := file.Source != "" || presence.Source
	contentSet := file.Content != "" || presence.Content

	if sourceSet && contentSet {
		return &diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "conflict in file block",
			Detail:   fmt.Sprintf("file %s has conflicting 'source' and 'content', choose only one", file.Destination),
		}
	}
	if sourceSet && file.Source == "" {
		return &diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "invalid source in file block",
			Detail:   fmt.Sprintf("file %s has an empty 'source', set a valid path or use 'content'", file.Destination),
		}
	}
	if !sourceSet && !contentSet {
		return &diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "conflict in file block",
			Detail:   fmt.Sprintf("file %s has neither 'source' nor 'content' set, set one (an empty 'content' creates an empty file)", file.Destination),
		}
	}

	return nil
}

func collectFilesToCreate(d *schema.ResourceData) ([]provisionFile, diag.Diagnostics) {
	var diags diag.Diagnostics
	files := make([]provisionFile, 0)
	if v, ok := d.GetOk("file"); ok {
		presence := configuredFilesForResource(d)
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
			if fileDiag := validateProvisionFile(file, presence[file.Destination]); fileDiag != nil {
				diags = append(diags, *fileDiag)
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
