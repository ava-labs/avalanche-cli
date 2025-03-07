locals {
  all_regions = [
    "us-east-1", "us-east-2", "us-west-1", "us-west-2",
    "ap-south-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-southeast-1", "ap-southeast-2",
    "ca-central-1",
    "eu-central-1", "eu-north-1", "eu-west-1", "eu-west-2", "eu-west-3",
    "sa-east-1"
  ]
}

packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.8"
      source  = "github.com/hashicorp/amazon"
    }
    googlecompute = {
      version = ">= 1.1.1"
      source = "github.com/hashicorp/googlecompute"
    }
  }
}

source "googlecompute" "ubuntu_gcp" {
  project_id      = "avalabs-experimental"
  source_image_family = "ubuntu-2004-lts"
  zone            = "us-central1-a"
  ssh_username    = "ubuntu"
  image_name     = "public-avalanchecli-ubuntu-focal-2004-docker"
  image_family   = "avalanchecli-ubuntu-2004"
  tags = ["public-avalanchecli","ubuntu-2004", "avaplatform"]
}

source "amazon-ebs" "ubuntu_amd64" {
  ami_name      = "public-avalanchecli-ubuntu-noble-24.04-docker-{{timestamp}}"
  ami_description = "Avalanche-CLI Ubuntu 24.04 Docker"
  instance_type = "t3.xlarge"
  region        = "us-east-1"

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }
  imds_support = "v2.0"
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*ubuntu-noble-24.04-amd64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }
  ssh_username = "ubuntu"
  ami_users = []
  ami_groups = ["all"]
  ami_regions = local.all_regions

  tags = {
    Name = "public-avalanchecli-ubuntu-noble-24.04-docker"
    Release = "ubuntu-24.04"
    Org = "avaplatform"
    Base_AMI_ID = "{{ .SourceAMI }}"
    Base_AMI_Name = "{{ .SourceAMIName }}"
    }
}

source "amazon-ebs" "ubuntu_arm64" {
  ami_name      = "public-avalanchecli-ubuntu-noble-24.04-docker-arm64-{{timestamp}}"
  ami_description = "Avalanche-CLI Ubuntu 24.04 Docker"
  instance_type = "t4g.xlarge"  # Adjusted for arm64 instance type
  region        = "us-east-1"
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }
  imds_support = "v2.0"
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*ubuntu-noble-24.04-arm64-server-*"  # Filter for arm64 AMIs (Ubuntu 24.04)
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }
  ssh_username = "ubuntu"
  ami_users = []
  ami_groups = ["all"]
  ami_regions = local.all_regions
  tags = {
    Name = "public-avalanchecli-ubuntu-noble-24.04-docker-arm64"
    Release = "ubuntu-24.04"
    Org = "avaplatform"
    Base_AMI_ID = "{{ .SourceAMI }}"
    Base_AMI_Name = "{{ .SourceAMIName }}"
  }
}

build {
  name    = "docker"
  sources = [
    "source.amazon-ebs.ubuntu_amd64",
    "source.amazon-ebs.ubuntu_arm64",
    "source.googlecompute.ubuntu_gcp"
  ]

    provisioner "shell" {
        inline = [
            "export DEBIAN_FRONTEND=noninteractive",
            "sudo add-apt-repository -y ppa:longsleep/golang-backports",
            "sudo apt-get -y update && sudo apt-get -y dist-upgrade && sudo apt-get -y install ca-certificates curl gcc git golang-go",
            "sudo install -m 0755 -d /etc/apt/keyrings && sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && sudo chmod a+r /etc/apt/keyrings/docker.asc",
            "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
            "sudo apt-get -y update && sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-compose",
            "sudo usermod -aG docker ubuntu",
            "sudo systemctl enable docker",
            "sudo systemctl start docker",
            "sudo chmod 666 /var/run/docker.sock",

        ]
    }
    # Install GLIBC 2.34 for AvalancheGo
    provisioner "shell" {
      inline = [
        "sudo apt-get install -y build-essential manpages-dev",
        "wget http://ftp.gnu.org/gnu/libc/glibc-2.34.tar.gz",
        "tar -xvzf glibc-2.34.tar.gz",
        "cd glibc-2.34 && mkdir build && cd build",
        "../configure --prefix=/opt/glibc-2.34",
        "make -j$(nproc)",
        "sudo make install",
        "echo 'export LD_LIBRARY_PATH=/opt/glibc-2.34/lib:$LD_LIBRARY_PATH' | sudo tee -a /etc/profile",
        "echo 'export PATH=/opt/glibc-2.34/bin:$PATH' | sudo tee -a /etc/profile",
        "source /etc/profile"
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
            "docker pull avaplatform/icm-relayer:v2.0.0-fuji",
            "docker pull golang:1.22.8-bullseye"
        ]
   }

    provisioner "shell" {
        inline = [
            "sudo rm -f /root/.ssh/authorized_keys && sudo rm -f /home/ubuntu/.ssh/authorized_keys"
            ]
    }
}


