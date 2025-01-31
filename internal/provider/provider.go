package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/SAP/terraform-provider-btp/internal/btpcli"
	"github.com/SAP/terraform-provider-btp/internal/version"
)

// New .
func New() provider.Provider {
	return NewWithClient(http.DefaultClient)
}

func NewWithClient(httpClient *http.Client) provider.Provider {
	return &btpcliProvider{httpClient: httpClient}
}

type btpcliProvider struct {
	httpClient          *http.Client
	betaFeaturesEnabled bool
}

// GetSchema
func (p *btpcliProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The Terraform provider for SAP BTP enables you to automate the provisioning, management, and configuration of resources on [SAP Business Technology Platform](https://account.hana.ondemand.com/). By leveraging this provider, you can simplify and streamline the deployment and maintenance of BTP services and applications.`,
		Attributes: map[string]schema.Attribute{
			"cli_server_url": schema.StringAttribute{
				MarkdownDescription: "The URL of the BTP CLI server (e.g. `https://cpcli.cf.eu10.hana.ondemand.com`).",
				Optional:            true, // TODO validate URL
			},
			"globalaccount": schema.StringAttribute{
				MarkdownDescription: "The subdomain of the global account in which you want to manage resources. To be found in the cockpit, in the global account view.",
				Required:            true, // TODO validate UUID
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Your user name, usually an e-mail address. This can also be sourced from the `BTP_USERNAME` environment variable.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Your password. Note that two-factor authentication is not supported. This can also be sourced from the `BTP_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"idp": schema.StringAttribute{
				MarkdownDescription: "The identity provider to be used for authentication (default: `sap.default`).",
				Optional:            true,
			},
		},
	}
}

// Provider schema struct
type providerData struct {
	CLIServerURL     types.String `tfsdk:"cli_server_url"`
	GlobalAccount    types.String `tfsdk:"globalaccount"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	IdentityProvider types.String `tfsdk:"idp"`
}

// Metadata returns the provider type name.
func (p *btpcliProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "btp"
}

func (p *btpcliProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	const unableToCreateClient = "unableToCreateClient"

	// Retrieve provider data from configuration
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	selectedCLIServerURL := btpcli.DefaultServerURL

	if !config.CLIServerURL.IsNull() {
		selectedCLIServerURL = config.CLIServerURL.ValueString()
	}

	u, err := url.Parse(selectedCLIServerURL) // TODO move to NewV2Client

	if err != nil {
		resp.Diagnostics.AddError(unableToCreateClient, fmt.Sprintf("%s", err))
		return
	}

	client := btpcli.NewClientFacade(btpcli.NewV2ClientWithHttpClient(p.httpClient, u))
	client.UserAgent = fmt.Sprintf("Terraform/%s terraform-provider-btp/%s", req.TerraformVersion, version.ProviderVersion)

	// User may provide an idp to the provider
	var idp string
	if config.IdentityProvider.IsUnknown() {
		resp.Diagnostics.AddWarning(unableToCreateClient, "Cannot use unknown value as identity provider")
		return
	}

	if config.IdentityProvider.IsNull() {
		idp = os.Getenv("BTP_IDP")
	} else {
		idp = config.IdentityProvider.ValueString()
	}

	// User must provide a username to the provider
	var username string
	if config.Username.IsUnknown() {
		resp.Diagnostics.AddWarning(unableToCreateClient, "Cannot use unknown value as username")
		return
	}

	if config.Username.IsNull() {
		username = os.Getenv("BTP_USERNAME")
	} else {
		username = config.Username.ValueString()
	}

	// User must provide a password to the provider
	var password string
	if config.Password.IsUnknown() {
		resp.Diagnostics.AddWarning(unableToCreateClient, "Cannot use unknown value as password")
		return
	}

	if config.Password.IsNull() {
		password = os.Getenv("BTP_PASSWORD")
	} else {
		password = config.Password.ValueString()
	}

	if len(username) == 0 || len(password) == 0 {
		resp.Diagnostics.AddError(unableToCreateClient, "globalaccount, username and password must be given.")
		return
	}

	if _, err = client.Login(ctx, btpcli.NewLoginRequestWithCustomIDP(idp, config.GlobalAccount.ValueString(), username, password)); err != nil {
		resp.Diagnostics.AddError(unableToCreateClient, fmt.Sprintf("%s", err))
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

// Resources - Defines provider resources
func (p *btpcliProvider) Resources(ctx context.Context) []func() resource.Resource {
	betaResources := []func() resource.Resource{
		newDirectoryRoleResource,
		newGlobalaccountRoleResource,
		newSubaccountRoleResource,
	}

	if !p.betaFeaturesEnabled {
		betaResources = nil
	}

	return append([]func() resource.Resource{
		newDirectoryResource,
		newDirectoryRoleCollectionAssignmentResource,
		newDirectoryRoleCollectionResource,
		newGlobalaccountResourceProviderResource,
		newGlobalaccountRoleCollectionAssignmentResource,
		newGlobalaccountRoleCollectionResource,
		newGlobalaccountTrustConfigurationResource,
		newSubaccountEntitlementResource,
		newSubaccountEnvironmentInstanceResource,
		newSubaccountResource,
		newSubaccountRoleCollectionAssignmentResource,
		newSubaccountRoleCollectionResource,
		newSubaccountServiceBindingResource,
		newSubaccountServiceInstanceResource,
		newSubaccountSubscriptionResource,
		newSubaccountTrustConfigurationResource,
	}, betaResources...)
}

// DataSources - Defines provider data sources
func (p *btpcliProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	betaDataSources := []func() datasource.DataSource{
		newDirectoryAppDataSource,
		newDirectoryAppsDataSource,
		newGlobalaccountAppDataSource,
		newGlobalaccountAppsDataSource,
		newGlobalaccountResourceProviderDataSource,
		newGlobalaccountResourceProvidersDataSource,
		newSubaccountServiceBrokerDataSource,
		newSubaccountServiceBrokersDataSource,
		newSubaccountServicePlatformDataSource,
		newSubaccountServicePlatformsDataSource,
	}

	if !p.betaFeaturesEnabled {
		betaDataSources = nil
	}

	return append([]func() datasource.DataSource{
		newDirectoryDataSource,
		newDirectoryEntitlementsDataSource,
		newDirectoryLabelsDataSource,
		newDirectoryRoleCollectionDataSource,
		newDirectoryRoleCollectionsDataSource,
		newDirectoryRoleDataSource,
		newDirectoryRolesDataSource,
		newDirectoryUserDataSource,
		newDirectoryUsersDataSource,
		newGlobalaccountDataSource,
		newGlobalaccountEntitlementsDataSource,
		newGlobalaccountRoleCollectionDataSource,
		newGlobalaccountRoleCollectionsDataSource,
		newGlobalaccountRoleDataSource,
		newGlobalaccountRolesDataSource,
		newGlobalaccountTrustConfigurationDataSource,
		newGlobalaccountTrustConfigurationsDataSource,
		newGlobalaccountUserDataSource,
		newGlobalaccountUsersDataSource,
		newRegionsDataSource,
		newSubaccountAppDataSource,
		newSubaccountAppsDataSource,
		newSubaccountDataSource,
		newSubaccountEntitlementsDataSource,
		newSubaccountEnvironmentInstanceDataSource,
		newSubaccountEnvironmentInstancesDataSource,
		newSubaccountEnvironmentsDataSource,
		newSubaccountLabelsDataSource,
		newSubaccountRoleCollectionDataSource,
		newSubaccountRoleCollectionsDataSource,
		newSubaccountRoleDataSource,
		newSubaccountRolesDataSource,
		newSubaccountServiceBindingDataSource,
		newSubaccountServiceBindingsDataSource,
		newSubaccountServiceInstanceDataSource,
		newSubaccountServiceInstancesDataSource,
		newSubaccountServiceOfferingDataSource,
		newSubaccountServiceOfferingsDataSource,
		newSubaccountServicePlanDataSource,
		newSubaccountServicePlansDataSource,
		newSubaccountSubscriptionDataSource,
		newSubaccountSubscriptionsDataSource,
		newSubaccountTrustConfigurationDataSource,
		newSubaccountTrustConfigurationsDataSource,
		newSubaccountUserDataSource,
		newSubaccountUsersDataSource,
		newSubaccountsDataSource,
		newWhoamiDataSource,
	}, betaDataSources...)
}
