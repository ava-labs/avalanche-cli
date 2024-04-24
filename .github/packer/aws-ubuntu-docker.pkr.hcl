locals {
  /*all_regions = [
    "us-east-1", "us-east-2", "us-west-1", "us-west-2",
    "ap-east-1", "ap-south-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-southeast-1", "ap-southeast-2",
    "ca-central-1",
    "cn-north-1", "cn-northwest-1",
    "eu-central-1", "eu-north-1", "eu-west-1", "eu-west-2", "eu-west-3",
    "me-south-1",
    "sa-east-1"
  ]
  */
  all_regions = [
    "us-east-1"
  ]
}

packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.8"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

source "amazon-ebs" "ubuntu" {
  ami_name      = "public-avalanchecli-ubuntu-jammy-22.04-docker-{{timestamp}}"
  ami_description = "AvalancheCLI Ubuntu 22.04 Docker"
  instance_type = "t3.xlarge"
  region        = "us-east-1"
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*ubuntu-jammy-22.04-amd64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }
  ssh_username = "ubuntu"
  ami_users = []
  ami_regions = local.all_regions
  
  tags = {
    Name = "public-avalanchecli-ubuntu-jammy-22.04-docker"
    Release = "ubuntu-22.04"
    Org = "avaplatform"
    Base_AMI_ID = "{{ .SourceAMI }}"
    Base_AMI_Name = "{{ .SourceAMIName }}"
    }
}

source "amazon-ebs" "ubuntu_arm64" {
  ami_name      = "public-avalanchecli-ubuntu-jammy-22.04-docker-arm64-{{timestamp}}"
  ami_description = "AvalancheCLI Ubuntu 22.04 Docker"
  instance_type = "t4g.xlarge"  # Adjust the instance type for arm64
  region        = "us-east-1"
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*ubuntu-jammy-22.04-arm64-server-*"  # Filter for arm64 AMIs
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }
  ssh_username = "ubuntu"
  ami_users = []
  ami_regions = local.all_regions
  tags = {
    Name = "public-avalanchecli-ubuntu-jammy-22.04-docker-arm64"
    Release = "ubuntu-22.04"
    Org = "avaplatform"
    Base_AMI_ID = "{{ .SourceAMI }}"
    Base_AMI_Name = "{{ .SourceAMIName }}"
  }
}


build {
  name    = "docker"
  sources = [
    "source.amazon-ebs.ubuntu",
    "source.amazon-ebs.ubuntu_arm64"
  ]

    provisioner "shell" {
        inline = [
            "export DEBIAN_FRONTEND=noninteractive",
            "sudo apt-get -y update && sudo apt-get -y dist-upgrade && sudo apt-get -y install ca-certificates curl",
            "sudo install -m 0755 -d /etc/apt/keyrings && sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && sudo chmod a+r /etc/apt/keyrings/docker.asc",
            "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
            "sudo apt-get -y update && sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-compose",
            "sudo usermod -aG docker ubuntu",
            "sudo systemctl enable docker || true",
            "sudo systemctl start docker || true",
            "sudo chmod 666 /var/run/docker.sock"
        ]
    }
    provisioner "shell" {
        inline = [
            "docker pull avaplatform/avalanchego",
            "docker pull grafana/promtail:3.0.0",
            "docker pull grafana/loki:3.0.0",
            "docker pull prom/node-exporter:v1.7.0",
            "docker pull grafana/grafana:10.4.1",
            "docker pull prom/prometheus:v2.51.2",
            "docker pull avaplatform/awm-relayer:v1.1.0"
        ]
   }

    provisioner "shell" {
        inline = [
            "sudo rm -f /root/.ssh/authorized_keys && sudo rm -f /home/ubuntu/.ssh/authorized_keys"
            ]
    }
}


