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

func customDiff(_ context.Context, d *schema.ResourceDiff, meta interface{}) error {
	if err := applyProviderConnectionDefaults(d, meta); err != nil {
		return err
	}

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

func applyProviderConnectionDefaults(d *schema.ResourceDiff, meta interface{}) error {
	defaults := SSHConnectionDefaults{
		Port:        "22",
		BastionPort: "22",
	}
	if config, ok := meta.(*Config); ok && config != nil {
		defaults = config.Connection
	}

	defaulted := []struct {
		attribute string
		value     interface{}
	}{
		{attribute: "port", value: defaults.Port},
		{attribute: "bastion_port", value: defaults.BastionPort},
		{attribute: "agent", value: defaults.Agent},
	}
	for _, item := range defaulted {
		if configured, known := rawAttributeConfigured(d.GetRawConfig(), item.attribute); known && configured {
			continue
		}
		if err := d.SetNew(item.attribute, item.value); err != nil {
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
			Description:  "The IP address or DNS hostname of the target server, without a port. Defaults to the provider 'host' value when omitted.",
			Type:         schema.TypeString,
			Optional:     true,
			ForceNew:     true,
			ValidateFunc: validateOptionalHost,
		},
		"port": {
			Description: "The SSH port to use on the target server. Defaults to the provider 'port' value, or 22 when unset.",
			Type:        schema.TypeString,
			Optional:    true,
			Computed:    true,
		},
		"bastion_host": {
			Description:  "The IP address or DNS hostname of the bastion host, without a port.",
			Type:         schema.TypeString,
			Optional:     true,
			ValidateFunc: validateOptionalHost,
		},
		"bastion_port": {
			Description: "The SSH port to use on the bastion host. Defaults to the provider 'bastion_port' value, or 22 when unset.",
			Type:        schema.TypeString,
			Optional:    true,
			Computed:    true,
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
			Description: "Whether to use an SSH agent for authentication. Defaults to the provider 'agent' value, or false when unset.",
			Type:        schema.TypeBool,
			Optional:    true,
			Computed:    true,
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
			Description: "Retry when authentication fails with 'no supported methods remain' instead of failing immediately.",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
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

type sshConnectionConfig struct {
	Host              string
	Port              string
	User              string
	Password          string
	PrivateKey        string
	HostUser          string
	HostPrivateKey    string
	BastionHost       string
	BastionPort       string
	BastionUser       string
	BastionPassword   string
	BastionPrivateKey string
	Agent             bool
}

func resolveConnectionConfig(d *schema.ResourceData, providerConfig *Config) sshConnectionConfig {
	connection := sshConnectionConfig{
		Host:              providerConfig.Connection.Host,
		Port:              providerConfig.Connection.Port,
		User:              providerConfig.Connection.User,
		Password:          providerConfig.Connection.Password,
		PrivateKey:        providerConfig.Connection.PrivateKey,
		BastionHost:       providerConfig.Connection.BastionHost,
		BastionPort:       providerConfig.Connection.BastionPort,
		BastionUser:       providerConfig.Connection.BastionUser,
		BastionPassword:   providerConfig.Connection.BastionPassword,
		BastionPrivateKey: providerConfig.Connection.BastionPrivateKey,
		Agent:             providerConfig.Connection.Agent,
	}

	setString := func(attribute string, target *string) {
		if value, ok := d.GetOk(attribute); ok {
			*target = value.(string)
		}
	}
	setString("host", &connection.Host)
	setString("user", &connection.User)
	setString("password", &connection.Password)
	setString("private_key", &connection.PrivateKey)
	setString("bastion_host", &connection.BastionHost)
	setString("bastion_user", &connection.BastionUser)
	setString("bastion_password", &connection.BastionPassword)
	setString("bastion_private_key", &connection.BastionPrivateKey)

	if resourceAttributeConfigured(d, "port") {
		connection.Port = d.Get("port").(string)
	}
	if resourceAttributeConfigured(d, "bastion_port") {
		connection.BastionPort = d.Get("bastion_port").(string)
	}
	if resourceAttributeConfigured(d, "agent") {
		connection.Agent = d.Get("agent").(bool)
	}

	// Deprecated per-host overrides take precedence over the generic values.
	if value := d.Get("host_user").(string); value != "" {
		connection.HostUser = value
	}
	if value := d.Get("host_private_key").(string); value != "" {
		connection.HostPrivateKey = value
	}

	return connection
}

func resourceAttributeConfigured(d *schema.ResourceData, attribute string) bool {
	for _, raw := range []cty.Value{d.GetRawConfig(), d.GetRawPlan(), d.GetRawState()} {
		if configured, known := rawAttributeConfigured(raw, attribute); known {
			return configured
		}
	}
	return false
}

func rawAttributeConfigured(raw cty.Value, attribute string) (bool, bool) {
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() || !raw.Type().HasAttribute(attribute) {
		return false, false
	}
	return !raw.GetAttr(attribute).IsNull(), true
}

func validateResource(d *schema.ResourceData, connection sshConnectionConfig) diag.Diagnostics {
	var diags diag.Diagnostics
	var commands []string

	timeout := d.Get("timeout").(string)
	retryDelay := d.Get("retry_delay").(string)

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
	if connection.Host == "" {
		return diag.FromErr(fmt.Errorf("host must be set on the resource or in the provider configuration"))
	}

	hostPrivateKey := connection.HostPrivateKey
	if hostPrivateKey == "" {
		hostPrivateKey = connection.PrivateKey
	}
	hostUser := connection.HostUser
	if hostUser == "" {
		hostUser = connection.User
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
		if hostUser == "" {
			return diag.FromErr(fmt.Errorf("user must be set on the resource or in the provider configuration when 'commands' is specified"))
		}
		if !connection.Agent && hostPrivateKey == "" && connection.Password == "" {
			return diag.FromErr(fmt.Errorf("'privateKey' must be set on the resource or in the provider configuration when 'commands' is specified and 'agent' is false and no 'password' is given"))
		}
	}
	if connection.Agent && (hostPrivateKey != "" || connection.PrivateKey != "" || connection.Password != "") {
		return diag.FromErr(fmt.Errorf("agent mode is enabled, not expecting a password or private key"))
	}
	return diags
}

func mainRun(_ context.Context, d *schema.ResourceData, m interface{}, onUpdate bool) diag.Diagnostics {
	config := m.(*Config)
	connection := resolveConnectionConfig(d, config)

	if diags := validateResource(d, connection); len(diags) > 0 {
		return diags
	}

	privateKeyPassphrase, _ := schema.EnvDefaultFunc("SSH_PRIVATE_KEY_PASSPHRASE", "")()
	bastionPrivateKeyPassphrase, _ := schema.EnvDefaultFunc("SSH_BASTION_PRIVATE_KEY_PASSPHRASE", "")()
	timeout := d.Get("timeout").(string)
	retryDelay := d.Get("retry_delay").(string)
	commandsAfterFileChanges := d.Get("commands_after_file_changes").(bool)

	var sshRetryConfig SSHRetryConfig

	sshRetryConfig.timeout, _ = time.ParseDuration(timeout)
	sshRetryConfig.retryDelay, _ = time.ParseDuration(retryDelay)
	sshRetryConfig.ignoreUnsupportedAuthMethods = d.Get("ignore_no_supported_methods_remain").(bool)

	hostUser := connection.HostUser
	if len(hostUser) == 0 {
		hostUser = connection.User
	}
	hostPrivateKey := connection.HostPrivateKey
	if len(hostPrivateKey) == 0 {
		hostPrivateKey = connection.PrivateKey
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
	ssh := &easyssh.MakeConfig{
		User:       hostUser,
		Server:     connection.Host,
		Port:       connection.Port,
		Key:        connection.PrivateKey,
		Passphrase: privateKeyPassphrase.(string),
		Proxy:      proxyFromEnvironment,
		Bastion: easyssh.DefaultConfig{
			User:       connection.User,
			Server:     connection.BastionHost,
			Passphrase: bastionPrivateKeyPassphrase.(string),
			Port:       connection.BastionPort,
		},
	}
	if connection.Password != "" {
		ssh.Password = connection.Password
	}
	if connection.BastionPassword != "" {
		ssh.Bastion.Password = connection.BastionPassword
	}
	if connection.BastionUser != "" {
		ssh.Bastion.User = connection.BastionUser
	}
	if hostPrivateKey != "" {
		ssh.Key = hostPrivateKey
	}
	if connection.PrivateKey != "" {
		ssh.Bastion.Key = connection.PrivateKey
	}
	if connection.BastionPrivateKey != "" {
		ssh.Bastion.Key = connection.BastionPrivateKey
	}

	if onUpdate && !d.HasChange("file") && !d.HasChange("commands") {
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
	if err := copyFiles(ctx, ssh, config, sshRetryConfig, createFiles); err != nil {
		return diag.FromErr(fmt.Errorf("copying files to remote: %w", err))
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
		config := m.(*Config)
		connection := resolveConnectionConfig(d, config)
		diags = validateResource(d, connection)
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

type sshCommandRunner interface {
	Run(command string, timeout ...time.Duration) (string, string, bool, error)
}

func runCommands(ctx context.Context, commands []string, ssh sshCommandRunner, sshRetryConfig SSHRetryConfig, m interface{}) (string, diag.Diagnostics, error) {
	var diags diag.Diagnostics
	var stdout, stderr string
	config := m.(*Config)

	for i := 0; i < len(commands); i++ {
		for {
			commandTimeout, timeoutErr := boundedCommandTimeout(ctx, sshRetryConfig.timeout)
			if timeoutErr != nil {
				diags = append(diags, commandFailureDiags(commands[i], timeoutErr, stdout, stderr)...)
				return stdout, diags, timeoutErr
			}

			var done bool
			var err error
			stdout, stderr, done, err = ssh.Run(commands[i], commandTimeout)
			_, _ = config.Debug("command: %s\ndone: %t\nstdout:\n%s\nstderr:\n%s\nerror: %v\n", commands[i], done, stdout, stderr, err)
			if err == nil {
				if done {
					break
				}
				timeoutErr = fmt.Errorf("execution of command '%s' timed out after %s", commands[i], commandTimeout)
				diags = append(diags, commandFailureDiags(commands[i], timeoutErr, stdout, stderr)...)
				return stdout, diags, timeoutErr
			}
			if !sshRetryConfig.ignoreUnsupportedAuthMethods && isUnsupportedAuthError(err) {
				diags = append(diags, commandFailureDiags(commands[i], err, stdout, stderr)...)
				return stdout, diags, err
			}

			select {
			case <-time.After(sshRetryConfig.retryDelay):
				// Retry

			case <-ctx.Done():
				_, _ = config.Debug("error: %v\n", err)
				diags = append(diags, commandFailureDiags(commands[i], fmt.Errorf("%s: %w", ctx.Err(), err), stdout, stderr)...)
				return stdout, diags, err
			}
		}
	}
	return stdout, diags, nil
}

func commandFailureDiags(command string, err error, stdout, stderr string) diag.Diagnostics {
	diags := diag.Diagnostics{{
		Severity: diag.Error,
		Summary:  fmt.Sprintf("execution of command '%s' failed: %s", command, err),
		Detail:   stdout,
	}}
	if stderr != "" {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "stderr output",
			Detail:   stderr,
		})
	}
	return diags
}

func isUnsupportedAuthError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no supported methods remain")
}

func boundedCommandTimeout(ctx context.Context, timeout time.Duration) (time.Duration, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return timeout, nil
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return 0, context.DeadlineExceeded
	}
	if remaining < timeout {
		return remaining, nil
	}
	return timeout, nil
}

func copyFiles(ctx context.Context, ssh *easyssh.MakeConfig, config *Config, sshRetryConfig SSHRetryConfig, createFiles []provisionFile) error {
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
				if err := ssh.WriteFile(src, srcStat.Size(), f.Destination); err != nil {
					_ = src.Close()
					_, _ = config.Debug("Failed to copy source file %s to remote file %s:%s:%s: %v\n", f.Source, ssh.Server, ssh.Port, f.Destination, err)
					return err
				}
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
				if err := runFileMetadataCommand(ctx, ssh, config, fmt.Sprintf("chmod %s \"%s\"", f.Permissions, f.Destination), sshRetryConfig.timeout); err != nil {
					return err
				}
			}
			// Owner
			if f.Owner != "" {
				if err := runFileMetadataCommand(ctx, ssh, config, fmt.Sprintf("chown %s \"%s\"", f.Owner, f.Destination), sshRetryConfig.timeout); err != nil {
					return err
				}
			}
			// Group
			if f.Group != "" {
				if err := runFileMetadataCommand(ctx, ssh, config, fmt.Sprintf("chgrp %s \"%s\"", f.Group, f.Destination), sshRetryConfig.timeout); err != nil {
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
			if !sshRetryConfig.ignoreUnsupportedAuthMethods && isUnsupportedAuthError(err) {
				return fmt.Errorf("file %s: %w", f.Destination, err)
			}
			select {
			case <-time.After(sshRetryConfig.retryDelay):
				// Retry
			case <-ctx.Done():
				return fmt.Errorf("file %s: %w (retry deadline: %v)", f.Destination, err, ctx.Err())
			}
		}
	}
	return nil
}

func runFileMetadataCommand(ctx context.Context, ssh *easyssh.MakeConfig, config *Config, command string, timeout time.Duration) error {
	commandTimeout, err := boundedCommandTimeout(ctx, timeout)
	if err != nil {
		return err
	}
	outStr, errStr, done, err := ssh.Run(command, commandTimeout)
	_, _ = config.Debug("%s: %v %v\n", command, outStr, errStr)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	if !done {
		return fmt.Errorf("%s timed out after %s", command, commandTimeout)
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
