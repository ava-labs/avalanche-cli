#!/usr/bin/env bash
# Setup SSH deploy key for private repository access

# Create .ssh directory if it doesn't exist
mkdir -p ~/.ssh

# Set proper permissions
chmod 700 ~/.ssh

# Upload the deploy key (this will be done by the Go code before calling this script)
# The key should already be at ~/.ssh/avalanche-deploy-key

# Set proper permissions for the deploy key
chmod 600 /home/ubuntu/.ssh/avalanche-deploy-key

# Configure SSH to use the deploy key
cat >> ~/.ssh/config <<'EOF'
Host github.com-avalanche
  HostName github.com
  User git
  IdentityFile /home/ubuntu/.ssh/avalanche-deploy-key
  IdentitiesOnly yes
  StrictHostKeyChecking accept-new
EOF

# Test the SSH connection
echo "Testing SSH connection to GitHub..."
ssh -T github.com-avalanche || true

echo "Deploy key setup completed successfully!"
