package dms

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/chnsz/golangsdk"
	"github.com/chnsz/golangsdk/openstack/common/tags"
	"github.com/chnsz/golangsdk/openstack/dms/v2/products"
	"github.com/chnsz/golangsdk/openstack/dms/v2/rabbitmq/instances"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/common"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/config"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/utils"
)

const lockKey = "DMS"

func ResourceDmsRabbitmqInstance() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDmsRabbitmqInstanceCreate,
		ReadContext:   resourceDmsRabbitmqInstanceRead,
		UpdateContext: resourceDmsRabbitmqInstanceUpdate,
		DeleteContext: resourceDmsRabbitmqInstanceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(50 * time.Minute),
			Update: schema.DefaultTimeout(50 * time.Minute),
			Delete: schema.DefaultTimeout(15 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"engine_version": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "3.7.17",
			},
			"storage_space": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			"storage_spec_code": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"access_user": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"password": {
				Type:      schema.TypeString,
				Sensitive: true,
				Required:  true,
				ForceNew:  true,
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"security_group_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"network_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"availability_zones": {
				// There is a problem with order of elements in Availability Zone list returned by RabbitMQ API.
				Type:          schema.TypeSet,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{"available_zones"},
				Elem:          &schema.Schema{Type: schema.TypeString},
				Set:           schema.HashString,
			},
			"product_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"maintain_begin": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"maintain_end": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"ssl_enable": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},
			"public_ip_id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"enterprise_project_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"tags": common.TagsSchema(),
			"engine": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"specification": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"enable_public_ip": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"used_storage_space": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"port": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"resource_spec_code": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"user_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"user_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"connect_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"management_connect_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"type": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"available_zones": {
				Type:         schema.TypeList,
				Optional:     true,
				Computed:     true,
				ForceNew:     true,
				Elem:         &schema.Schema{Type: schema.TypeString},
				AtLeastOneOf: []string{"available_zones", "availability_zones"},
				Deprecated:   "available_zones has deprecated, please use \"availability_zones\" instead.",
			},
			// Typo, it is only kept in the code, will not be shown in the docs.
			"manegement_connect_address": {
				Type:       schema.TypeString,
				Computed:   true,
				Deprecated: "typo in manegement_connect_address, please use \"management_connect_address\" instead.",
			},
		},
	}
}

func getRabbitMQProductDetail(cfg *config.Config, d *schema.ResourceData) (*products.ProductInfo, error) {
	productRsp, err := getProducts(cfg, cfg.GetRegion(d), "rabbitmq")
	if err != nil {
		return nil, fmt.Errorf("error querying product detail, please check product_id, error: %s", err)
	}

	productID := d.Get("product_id").(string)
	engineVersion := d.Get("engine_version").(string)

	for _, ps := range productRsp.Hourly {
		if ps.Version != engineVersion {
			continue
		}
		for _, v := range ps.Values {
			for _, detail := range v.Details {
				// All informations of product for single instance type and the kafka engine type are stored in the
				// detail structure.
				if v.Name == "single" {
					if detail.ProductID == productID {
						return &products.ProductInfo{
							Storage:          detail.Storage,
							ProductID:        detail.ProductID,
							SpecCode:         detail.SpecCode,
							IOs:              detail.IOs,
							AvailableZones:   detail.AvailableZones,
							UnavailableZones: detail.UnavailableZones,
						}, nil
					}
				} else {
					for _, product := range detail.ProductInfos {
						if product.ProductID == productID {
							return &product, nil
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("can not found product detail base on product_id: %s", productID)
}

func resourceDmsRabbitmqInstanceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config.MutexKV.Lock(lockKey)
	defer config.MutexKV.Unlock(lockKey)

	cfg := meta.(*config.Config)
	region := cfg.GetRegion(d)
	client, err := cfg.DmsV2Client(region)
	if err != nil {
		return diag.Errorf("error initializing DMS RabbitMQ(v2) client: %s", err)
	}

	var availableZones []string // Available zones IDs
	azIDs, ok := d.GetOk("available_zones")
	if ok {
		availableZones = utils.ExpandToStringList(azIDs.([]interface{}))
	} else {
		// convert the codes of the availability zone into ids
		azCodes := d.Get("availability_zones").(*schema.Set)
		availableZones, err = getAvailableZoneIDByCode(cfg, region, azCodes.List())
		if err != nil {
			return diag.FromErr(err)
		}
	}

	storageSpace := d.Get("storage_space").(int)
	if storageSpace == 0 {
		product, err := getRabbitMQProductDetail(cfg, d)
		if err != nil || product == nil {
			return diag.Errorf("failed to query DMS RabbitMQ product details: %s", err)
		}

		defaultStorageSpace, err := strconv.ParseInt(product.Storage, 10, 32)
		if err != nil {
			return diag.Errorf("failed to create RabbitMQ instance, error parsing storage_space to int %v: %s",
				product.Storage, err)
		}
		storageSpace = int(defaultStorageSpace)
	}

	createOpts := &instances.CreateOps{
		Name:                d.Get("name").(string),
		Description:         d.Get("description").(string),
		Engine:              engineRabbitMQ,
		EngineVersion:       d.Get("engine_version").(string),
		StorageSpace:        storageSpace,
		AccessUser:          d.Get("access_user").(string),
		VPCID:               d.Get("vpc_id").(string),
		SecurityGroupID:     d.Get("security_group_id").(string),
		SubnetID:            d.Get("network_id").(string),
		AvailableZones:      availableZones,
		ProductID:           d.Get("product_id").(string),
		MaintainBegin:       d.Get("maintain_begin").(string),
		MaintainEnd:         d.Get("maintain_end").(string),
		SslEnable:           d.Get("ssl_enable").(bool),
		StorageSpecCode:     d.Get("storage_spec_code").(string),
		EnterpriseProjectID: common.GetEnterpriseProjectID(d, cfg),
	}

	if pubIpID, ok := d.GetOk("public_ip_id"); ok {
		createOpts.EnablePublicIP = true
		createOpts.PublicIpID = pubIpID.(string)
	}

	// set tags
	if tagRaw := d.Get("tags").(map[string]interface{}); len(tagRaw) > 0 {
		createOpts.Tags = utils.ExpandResourceTags(tagRaw)
	}

	log.Printf("[DEBUG] Create DMS RabbitMQ instance Options: %#v", createOpts)
	// Add password here so it wouldn't go in the above log entry
	createOpts.Password = d.Get("password").(string)

	v, err := instances.Create(client, createOpts).Extract()
	if err != nil {
		return diag.Errorf("error creating DMS RabbitMQ instance: %s", err)
	}
	log.Printf("[INFO] Creating RabbitMQ instance, ID: %s", v.InstanceID)

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"CREATING"},
		Target:       []string{"RUNNING"},
		Refresh:      rabbitmqInstanceStateRefreshFunc(client, v.InstanceID),
		Timeout:      d.Timeout(schema.TimeoutCreate),
		Delay:        300 * time.Second,
		MinTimeout:   10 * time.Second,
		PollInterval: 15 * time.Second,
	}

	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return diag.Errorf("error waiting for RabbitMQ instance (%s) to be ready: %s", v.InstanceID, err)
	}

	// Store the instance ID now
	d.SetId(v.InstanceID)

	return resourceDmsRabbitmqInstanceRead(ctx, d, meta)
}

func resourceDmsRabbitmqInstanceRead(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	cfg := meta.(*config.Config)
	region := cfg.GetRegion(d)

	client, err := cfg.DmsV2Client(region)
	if err != nil {
		return diag.Errorf("error initializing DMS RabbitMQ(v2) client: %s", err)
	}

	v, err := instances.Get(client, d.Id()).Extract()
	if err != nil {
		return common.CheckDeletedDiag(d, err, "DMS RabbitMQ instance")
	}

	log.Printf("[DEBUG] DMS RabbitMQ instance %+v", v)

	azIDs := v.AvailableZones
	asCodes, err := getAvailableZoneCodeByID(cfg, region, azIDs)
	mErr := multierror.Append(nil, err)

	d.SetId(v.InstanceID)
	mErr = multierror.Append(mErr,
		d.Set("region", region),
		d.Set("name", v.Name),
		d.Set("description", v.Description),
		d.Set("engine", v.Engine),
		d.Set("engine_version", v.EngineVersion),
		d.Set("specification", v.Specification),
		// storage_space indicates total_storage_space while creating
		// set value of total_storage_space to storage_space to keep consistent
		d.Set("storage_space", v.TotalStorageSpace),

		d.Set("vpc_id", v.VPCID),
		d.Set("security_group_id", v.SecurityGroupID),
		d.Set("network_id", v.SubnetID),
		d.Set("available_zones", v.AvailableZones),
		d.Set("availability_zones", asCodes),
		d.Set("product_id", v.ProductID),
		d.Set("maintain_begin", v.MaintainBegin),
		d.Set("maintain_end", v.MaintainEnd),
		d.Set("enable_public_ip", v.EnablePublicIP),
		d.Set("public_ip_id", v.PublicIPID),
		d.Set("ssl_enable", v.SslEnable),
		d.Set("storage_spec_code", v.StorageSpecCode),
		d.Set("enterprise_project_id", v.EnterpriseProjectID),
		d.Set("used_storage_space", v.UsedStorageSpace),
		d.Set("connect_address", v.ConnectAddress),
		d.Set("management_connect_address", v.ManagementConnectAddress),
		d.Set("manegement_connect_address", v.ManagementConnectAddress),
		d.Set("port", v.Port),
		d.Set("status", v.Status),
		d.Set("resource_spec_code", v.ResourceSpecCode),
		d.Set("user_id", v.UserID),
		d.Set("user_name", v.UserName),
		d.Set("type", v.Type),
		d.Set("access_user", v.AccessUser),
	)

	// set tags
	if resourceTags, err := tags.Get(client, engineRabbitMQ, d.Id()).Extract(); err == nil {
		tagMap := utils.TagsToMap(resourceTags.Tags)
		err = d.Set("tags", tagMap)
		if err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf("error saving tags to state for DMS RabbitMQ instance (%s): %s", d.Id(), err))
		}
	} else {
		log.Printf("[WARN] error fetching tags of DMS RabbitMQ instance (%s): %s", d.Id(), err)
		e := fmt.Errorf("error fetching tags of DMS RabbitMQ instance (%s): %s", d.Id(), err)
		mErr = multierror.Append(mErr, e)
	}

	if mErr.ErrorOrNil() != nil {
		return diag.Errorf("failed to set attributes for DMS RabbitMQ instance: %s", mErr)
	}

	return nil
}

func resourceDmsRabbitmqInstanceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config.MutexKV.Lock(lockKey)
	defer config.MutexKV.Unlock(lockKey)

	cfg := meta.(*config.Config)
	client, err := cfg.DmsV2Client(cfg.GetRegion(d))
	if err != nil {
		return diag.Errorf("error initializing DMS RabbitMQ(v2) client: %s", err)
	}

	var mErr *multierror.Error
	if d.HasChanges("name", "description", "maintain_begin", "maintain_end",
		"security_group_id", "public_ip_id", "enterprise_project_id") {
		description := d.Get("description").(string)
		updateOpts := instances.UpdateOpts{
			Description:         &description,
			MaintainBegin:       d.Get("maintain_begin").(string),
			MaintainEnd:         d.Get("maintain_end").(string),
			SecurityGroupID:     d.Get("security_group_id").(string),
			EnterpriseProjectID: d.Get("enterprise_project_id").(string),
		}

		if d.HasChange("name") {
			updateOpts.Name = d.Get("name").(string)
		}

		if d.HasChange("public_ip_id") {
			if pubIpID, ok := d.GetOk("public_ip_id"); ok {
				enablePublicIP := true
				updateOpts.EnablePublicIP = &enablePublicIP
				updateOpts.PublicIpID = pubIpID.(string)
			} else {
				enablePublicIP := false
				updateOpts.EnablePublicIP = &enablePublicIP
			}
		}

		err = instances.Update(client, d.Id(), updateOpts).Err
		if err != nil {
			e := fmt.Errorf("error updating DMS RabbitMQ Instance: %s", err)
			mErr = multierror.Append(mErr, e)
		}
	}

	if d.HasChange("tags") {
		// update tags
		tagErr := utils.UpdateResourceTags(client, d, engineRabbitMQ, d.Id())
		if tagErr != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("error updating tags of DMS RabbitMQ instance: %s, err: %s",
				d.Id(), tagErr))
		}
	}

	if d.HasChange("product_id") {
		err = resizeRabbitMQInstance(ctx, d, meta, engineRabbitMQ)
		if err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}
	if mErr.ErrorOrNil() != nil {
		return diag.Errorf("error while updating DMS RabbitMQ instances, %s", mErr)
	}

	return resourceDmsRabbitmqInstanceRead(ctx, d, meta)
}

func resizeRabbitMQInstance(ctx context.Context, d *schema.ResourceData, meta interface{}, engineType string) error {
	cfg := meta.(*config.Config)
	client, err := cfg.DmsV2Client(cfg.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error initializing DMS(v2) client: %s", err)
	}

	product, err := getRabbitMQProductDetail(cfg, d)
	if err != nil {
		return fmt.Errorf("failed to resize RabbitMQ instance, query product details error: %s", err)
	}

	storage, err := strconv.Atoi(product.Storage)
	if err != nil {
		return fmt.Errorf("failed to resize RabbitMQ instance, error parsing storage_space to int %v: %s",
			product.Storage, err)
	}

	resizeOpts := instances.ResizeInstanceOpts{
		NewSpecCode:     product.SpecCode,
		NewStorageSpace: storage,
	}
	log.Printf("[DEBUG] Resize DMS %s instance option : %#v", engineType, resizeOpts)

	_, err = instances.Resize(client, d.Id(), resizeOpts)
	if err != nil {
		return fmt.Errorf("resize RabbitMQ instance failed: %s", err)
	}

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"EXTENDING", "PENDING"},
		Target:       []string{"RUNNING"},
		Refresh:      rabbitMQResizeStateRefresh(client, d.Id(), product.ProductID),
		Timeout:      d.Timeout(schema.TimeoutUpdate),
		Delay:        180 * time.Second,
		PollInterval: 15 * time.Second,
	}
	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("error waiting for RabbitMQ instance (%s) to resize: %v", d.Id(), err)
	}

	return nil
}

func rabbitMQResizeStateRefresh(client *golangsdk.ServiceClient, instanceID, productID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		v, err := instances.Get(client, instanceID).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				return v, "failed", fmt.Errorf("unable to resize RabbitMQ instance which has been deleted: %s",
					instanceID)
			}
			return nil, "failed", err
		}
		if v.Status == "RUNNING" && v.ProductID != productID {
			return v, "PENDING", nil
		}
		return v, v.Status, nil
	}
}

func resourceDmsRabbitmqInstanceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	cfg := meta.(*config.Config)
	client, err := cfg.DmsV2Client(cfg.GetRegion(d))
	if err != nil {
		return diag.Errorf("error initializing DMS RabbitMQ(v2) client: %s", err)
	}

	err = instances.Delete(client, d.Id()).ExtractErr()
	if err != nil {
		return common.CheckDeletedDiag(d, err, "failed to delete DMS RabbitMQ instance")
	}

	// Wait for the instance to delete before moving on.
	log.Printf("[DEBUG] Waiting for DMS RabbitMQ instance (%s) to be deleted", d.Id())

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"DELETING", "RUNNING", "ERROR"}, // Status may change to ERROR on deletion.
		Target:       []string{"DELETED"},
		Refresh:      rabbitmqInstanceStateRefreshFunc(client, d.Id()),
		Timeout:      d.Timeout(schema.TimeoutDelete),
		Delay:        90 * time.Second,
		MinTimeout:   5 * time.Second,
		PollInterval: 15 * time.Second,
	}

	_, err = stateConf.WaitForStateContext(ctx)
	if err != nil {
		return diag.Errorf("error waiting for DMS RabbitMQ instance (%s) to be deleted: %s", d.Id(), err)
	}

	log.Printf("[DEBUG] DMS RabbitMQ instance %s has been deleted", d.Id())
	d.SetId("")
	return nil
}

func rabbitmqInstanceStateRefreshFunc(client *golangsdk.ServiceClient, instanceID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		v, err := instances.Get(client, instanceID).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				return v, "DELETED", nil
			}
			return nil, "", err
		}

		return v, v.Status, nil
	}
}
