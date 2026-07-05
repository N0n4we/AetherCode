variable "tencentcloud_region" {
  description = "Tencent Cloud region for the AetherCode TKE runtime."
  type        = string
  default     = "ap-shanghai"
}

variable "node_pool_instance_type" {
  description = "CVM instance type used if the node pool is scaled above zero."
  type        = string
  default     = "SA9.MEDIUM2"
}

variable "node_pool_max_size" {
  description = "Maximum nodes for the TKE node pool."
  type        = number
  default     = 2
}

variable "kube_api_allowed_cidrs" {
  description = "CIDR ranges allowed to access the public TKE Kubernetes API endpoint."
  type        = list(string)
  default     = ["188.253.117.150/32", "45.137.183.42/32"]
}
