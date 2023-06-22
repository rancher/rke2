output "master_ips" {
  value       = module.master.master_ips
  description = "The public IP of the AWS node"
}

output "worker_ips" {
  value       = module.worker.worker_ips
  description = "The public IP of the AWS node"
}

output "windows_worker_ips" {
  value       = module.windows_worker.windows_worker_ips
  description = "The public IP of the AWS node"
}

output "kubeconfig" {
  value = module.master.kubeconfig
  description = "kubeconfig of the cluster created"
}