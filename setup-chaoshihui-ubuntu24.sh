#!/bin/bash
set -e

DEB_PATH="/tmp/chaoshihui-2.2.3-linux-amd64.deb"
AUTO_PROXY_FILE="/etc/profile.d/chaoshihui-proxy.sh"
AUTO_FREEZE_SCRIPT="/usr/local/bin/chaoshihui-autofreeze.sh"
AUTO_START_SCRIPT="/usr/local/bin/start-chaoshihui.sh"
AUTO_START_DESKTOP="/root/.config/autostart/chaoshihui.desktop"

echo "== 更新系统 =="
apt update
apt -y upgrade

echo "== 安装 XFCE + XRDP + 常用工具 =="
DEBIAN_FRONTEND=noninteractive apt install -y \
  xfce4 xfce4-goodies xrdp dbus-x11 \
  curl wget htop net-tools psmisc

echo "== 设置 XRDP 使用 XFCE =="
echo xfce4-session > /etc/skel/.xsession
echo xfce4-session > /root/.xsession

echo "== 启动 XRDP =="
systemctl enable xrdp
systemctl restart xrdp

echo "== 安装 chaoshihui =="
if [ ! -f "$DEB_PATH" ]; then
  echo "未找到安装包: $DEB_PATH"
  exit 1
fi

apt install -y "$DEB_PATH" || apt --fix-broken install -y

echo "== 创建 2G swap =="
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
fi
swapon /swapfile || true
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab

echo "== 优化 swap 参数 =="
sysctl vm.swappiness=10
grep -q 'vm.swappiness=10' /etc/sysctl.conf || echo 'vm.swappiness=10' >> /etc/sysctl.conf

echo "== 写入全局代理环境变量 =="
cat > "$AUTO_PROXY_FILE" <<'EOF'
export http_proxy=http://127.0.0.1:7892
export https_proxy=http://127.0.0.1:7892
export all_proxy=socks5://127.0.0.1:7892
export HTTP_PROXY=http://127.0.0.1:7892
export HTTPS_PROXY=http://127.0.0.1:7892
export ALL_PROXY=socks5://127.0.0.1:7892
export no_proxy=127.0.0.1,localhost
export NO_PROXY=127.0.0.1,localhost
EOF
chmod 644 "$AUTO_PROXY_FILE"

echo "== 创建自动冻结脚本 =="
cat > "$AUTO_FREEZE_SCRIPT" <<'EOF'
#!/bin/bash
set -e

LOG_FILE=/tmp/chaoshihui-autofreeze.log
echo "[$(date '+%F %T')] start autofreeze" >> "$LOG_FILE"

sleep 20

GUI_PID="$(pgrep -x chaoshihui | head -n1 || true)"
CORE_PID="$(pgrep -f ChaoShiHuiCore | head -n1 || true)"

echo "[$(date '+%F %T')] GUI_PID=$GUI_PID CORE_PID=$CORE_PID" >> "$LOG_FILE"

if [ -n "$GUI_PID" ] && [ -n "$CORE_PID" ]; then
  kill -STOP "$GUI_PID" || true
  echo "[$(date '+%F %T')] freezed GUI pid $GUI_PID" >> "$LOG_FILE"
else
  echo "[$(date '+%F %T')] skip freeze, process not ready" >> "$LOG_FILE"
fi
EOF
chmod +x "$AUTO_FREEZE_SCRIPT"

echo "== 创建启动脚本 =="
cat > "$AUTO_START_SCRIPT" <<'EOF'
#!/bin/bash
set -e

pgrep -x chaoshihui >/dev/null 2>&1 && exit 0

nohup chaoshihui >/tmp/chaoshihui.log 2>&1 &
nohup /usr/local/bin/chaoshihui-autofreeze.sh >/tmp/chaoshihui-autofreeze-run.log 2>&1 &
EOF
chmod +x "$AUTO_START_SCRIPT"

echo "== 创建 root 图形登录自启动 =="
mkdir -p /root/.config/autostart
cat > "$AUTO_START_DESKTOP" <<EOF
[Desktop Entry]
Type=Application
Name=ChaoShiHui Auto Start
Exec=$AUTO_START_SCRIPT
X-GNOME-Autostart-enabled=true
EOF

echo "== 添加便捷命令 =="
cat >> /root/.bashrc <<'EOF'

alias proxyfreeze='kill -STOP $(pgrep -x chaoshihui | head -n1)'
alias proxywake='kill -CONT $(pgrep -x chaoshihui | head -n1)'
alias proxystatus='ps -fp $(pgrep -x chaoshihui | head -n1) $(pgrep -f ChaoShiHuiCore | head -n1); echo ====; ss -lntp | grep 7892'
EOF

echo "== 完成 =="
echo "现在请重启系统:"
echo "reboot"
echo
echo "重启后用远程桌面连接 :3389"
echo "登录图形界面后，chaoshihui 会自动启动，并在约 20 秒后自动冻结 GUI。"
echo
echo "常用命令："
echo "  proxyfreeze   # 手动冻结 GUI"
echo "  proxywake     # 恢复 GUI"
echo "  proxystatus   # 查看代理状态"
