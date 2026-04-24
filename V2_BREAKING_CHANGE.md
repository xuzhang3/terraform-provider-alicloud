
# Resource

## Removed property
* `alicloud_click_house_db_cluster`
  - Removing the removed `db_cluster_access_white_list.db_cluster_ip_array_attribute` field.
* `alicloud_amqp_queue`
  - Removing the removed `exclusive_state` field.
* `alicloud_cms_alarm`
  - Removing the removed `operator`, `statistics`, `threshold`, `triggered_count`, `notify_type` field.
* `alicloud_eci_container_group`
  - Removing the removed `eci_security_context` field.
* `alicloud_disk` / `alicloud_ecs_disk`
  - Removing the removed `dedicated_block_storage_cluster_id` field.
* `alicloud_slb_master_slave_server_group`
  - Removing the removed `servers.is_backup` field.
* `alicloud_slb` / `alicloud_slb_load_balancer`
  - Removing the removed `internet` field.
* `alicloud_slb_listener`
  - Removing the removed `lb_port`, `instance_port`, `lb_protocol` field.
* `alicloud_resource_manager_role`
  - Removing the removed `create_date` field.
* `alicloud_resource_manager_resource_group`
  - Removing the removed `create_date` field.
* `alicloud_resource_manager_policy_version`
  - Removing the removed `create_date`, `version_id` field.
* `alicloud_pvtz_zone`
  - Removing the removed `creation_time`, `update_time` field.
* `alicloud_kvstore_instance`
  - Removing the removed `modify_mode` field.
* `alicloud_instance`
  - Removing the removed `io_optimized`, `subnet_id` field.
* `alicloud_express_connect_virtual_border_router`
  - Removing the removed `include_cross_account_vbr` field.
* `alicloud_hbr_vault`
  - Removing the removed `redundancy_type` field.
* `alicloud_ros_stack_group`
  - Removing the removed `account_ids`, `operation_description`, `operation_preferences`, `region_ids` field.
* `alicloud_nat_gateway`
  - Removing the removed `bandwidth_packages`, `bandwidth_packages.ip_count`, `bandwidth_packages.bandwidth`, `bandwidth_packages.zone`, `bandwidth_packages.public_ip_addresses`, `bandwidth_package_ids`, `spec` field.
* `alicloud_vpc_nat_ip`
    - Removing removed field `nat_ip_cidr_id`
* `alicloud_cloud_firewall_instance`
  - Removing removed field `cfw_service`
* `alicloud_lindorm_instance` 
  - Remove `core_num`, `group_name`, `phoenix_node_count`, `phoenix_node_specification`, `upgrade_type`
* `alicloud_sae_application` 
  - Remove `version_id`, `mount_desc`, `mount_host`, `nas_id`

# Data Resource

## Removed property
* `alicloud_click_house_db_clusters`
  -  Removing the removed `clusters.db_cluster_access_white_list.db_cluster_ip_array_attribute` field.
* `alicloud_cloud_firewall_control_policies` 
  - Removing the removed `source_ip` field.
* `alicloud_config_rules`
  - Removing the removed `multi_account`, `member_id`, `message_type` field.
* `alicloud_eips` / `alicloud_eip_addresses`
  - Removing the removed `in_use` field.
* `alicloud_hbase_zones`
  - Removing the removed `multi`, `zones.multi_zone_ids` field.
* `alicloud_kms_key_versions`
  - Removing the removed `versions.creation_date` field.
* `alicloud_ots_instances`
  - Removing the removed `instances.network`, `instances.entity_quota` field.
* `alicloud_pvtz_zones`
  - Removing the removed `zones.creation_time`, `zones.update_time` field.
* `alicloud_ram_policies` 
  - Removing the removed `policies.user_name` field.
* `alicloud_ram_users`
  - Removing the removed `users.last_login_date` field.
* `alicloud_slbs` / `alicloud_slb_load_balancers`
  - Removing the removed `master_availability_zone`, `slave_availability_zone` field.
* `alicloud_slb_master_slave_server_groups`
  - Removing the removed `groups.servers.is_backup` field.
