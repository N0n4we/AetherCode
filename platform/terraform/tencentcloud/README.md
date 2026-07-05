# Tencent Cloud Platform

This directory manages the Tencent Cloud resources used to run AetherCode on TKE.

Currently tracked resources:

- VPC `vpc-kdegvs6i` (`aether`), existing node/Pod subnets, and the
  `ap-shanghai-5` Pod ENI subnet required by nodes in that zone
- NAT Gateway, EIP, and default route for private node outbound internet access
- The default security group `sg-bvlotzok` and its IPv4/IPv6 default rule set
- TKE cluster `cls-26zqizrl` (`aether`)
- Public TKE Kubernetes API endpoint restricted by `kube_api_allowed_cidrs`
- A Terraform-managed public EIP-backed application CLB for relay and Open
  WebUI test access. The Kubernetes Services expose fixed NodePorts; Terraform
  owns the public application CLB to avoid Tencent Cloud default CLB domain
  blocking.
- Zero-size TKE node pool `np-8ldph9uj`
- SSH key pair `aether_tf_node_pool`
- `AS_QCSRole` and the Auto Scaling policies required by TKE node pools

The public TCR Personal repository
`ccr.ccs.tencentyun.com/aethercode-100034871923/router` currently exists for
the TKE runtime image, but TencentCloud Terraform provider resources cover TCR
Enterprise instances rather than Personal repositories, so that repository is
documented here instead of managed by Terraform.

Most resources already exist. The `imports.tf` file lets a fresh local state
adopt them with `terraform plan` / `terraform apply`; the `aether-pods-az5`
subnet is created by Terraform so TKE can allocate VPC-CNI Pod IPs for nodes in
`ap-shanghai-5`.

The node pool is configured with `min_size = 0` and `max_size = 2`. Set
`node_pool_max_size = 0` when the environment should be parked with no CVM node
capacity.

Use the tccli default profile with Terraform:

```sh
TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform plan -input=false
```

The VPC's default route table is referenced through `tencentcloud_vpc.aether`
and receives a managed default route through the NAT Gateway.
