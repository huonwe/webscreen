```
#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Ensure the script is run with root privileges
if [ "$EUID" -ne 0 ]; then
  echo "❌ Error: Please run this script with sudo (e.g., sudo bash setup_wayland_input.sh)"
  exit 1
fi

# Get the actual regular user who invoked sudo
TARGET_USER=${SUDO_USER:-$(whoami)}

if [ "$TARGET_USER" = "root" ]; then
  echo "❌ Error: Please do not run directly as the root user. Run as a regular user via sudo."
  exit 1
fi

echo "🚀 Starting to configure Wayland virtual input environment for user [$TARGET_USER]..."

# ==========================================
# 1. Configure uinput user group and permissions
# ==========================================
echo -e "\n[1/4] Configuring uinput user group..."
groupadd -f uinput
usermod -aG uinput "$TARGET_USER"
usermod -aG input "$TARGET_USER"
usermod -aG video "$TARGET_USER"

echo -e "\n[2/4] Writing /dev/uinput udev rules..."
UDEV_RULE_FILE="/etc/udev/rules.d/99-uinput.rules"
echo 'KERNEL=="uinput", MODE="0660", GROUP="uinput", OPTIONS+="static_node=uinput"' > "$UDEV_RULE_FILE"

# Apply udev rules immediately
udevadm control --reload-rules
udevadm trigger

# ==========================================
# 2. Install and configure seatd
# ==========================================
echo -e "\n[3/4] Checking and installing seatd..."
if ! command -v seatd &> /dev/null; then
    if command -v apt &> /dev/null; then
        apt update && apt install -y seatd
    elif command -v pacman &> /dev/null; then
        pacman -S --noconfirm seatd
    else
        echo "⚠️ No apt or pacman package manager detected. Please install seatd manually!"
        exit 1
    fi
else
    echo "✅ seatd is already installed, skipping download."
fi

echo -e "\n[4/4] Configuring seatd system service..."
# Different Linux distributions use different default groups for seatd 
# (Debian uses _seatd, Arch uses seat). We add the user to both for compatibility.
getent group _seatd >/dev/null && usermod -aG _seatd "$TARGET_USER"
getent group seat >/dev/null && usermod -aG seat "$TARGET_USER"

# Enable and restart the seatd service
systemctl enable seatd
systemctl restart seatd

echo -e "\n🎉 Configuration complete!"
echo "================================================================="
echo "⚠️  CRITICAL LAST STEP:"
echo "Because the system assigned new user group permissions, they are not yet active in the current terminal."
echo "Please do ONE of the following:"
echo "  👉 1. Completely log out of your current SSH session and log back in."
echo "  👉 2. Or simply reboot the server: sudo reboot"
echo "================================================================="
echo "💡 After reconnecting/rebooting, ensure the Sway environment variables in your Go program include:"
echo "   export SEATD_SOCK=/run/seatd.sock"
echo "================================================================="
```