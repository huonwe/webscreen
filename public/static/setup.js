document.addEventListener('DOMContentLoaded', () => {
  const form = document.getElementById('setup-form');
  const usernameInput = document.getElementById('username');
  const passwordInput = document.getElementById('password');
  const confirmInput = document.getElementById('confirm-password');
  const togglePwdBtn = document.getElementById('toggle-pwd');
  const generateBtn = document.getElementById('generate-btn');
  const strengthBar = document.getElementById('strength-bar');
  const strengthText = document.getElementById('strength-text');
  const alertBox = document.getElementById('alert-box');
  const submitBtn = document.getElementById('submit-btn');
  const btnText = submitBtn.querySelector('.btn-text');
  const spinner = submitBtn.querySelector('.spinner');

  // 使用 Tailwind 类名切换 Alert 样式
  function showAlert(message, type = 'error') {
    alertBox.textContent = message;
    if (type === 'success') {
      alertBox.className = 'mb-5 px-4 py-3 rounded-lg text-sm border bg-emerald-500/10 border-emerald-500/30 text-emerald-300';
    } else {
      alertBox.className = 'mb-5 px-4 py-3 rounded-lg text-sm border bg-red-500/10 border-red-500/30 text-red-300';
    }
  }

  function hideAlert() {
    alertBox.className = 'hidden';
  }

  // 计算密码强度并更新 Tailwind 颜色类
  function evaluateStrength(pwd) {
    if (!pwd) {
      strengthBar.className = 'h-full w-0 transition-all duration-300 rounded-full';
      strengthText.textContent = 'Enter a strong password to prevent brute-force attacks over the public network';
      strengthText.className = 'text-xs text-zinc-400 mt-1.5';
      return;
    }

    let score = 0;
    if (pwd.length >= 8) score++;
    if (pwd.length >= 12) score++;
    if (/[a-z]/.test(pwd) && /[A-Z]/.test(pwd)) score++;
    if (/\d/.test(pwd)) score++;
    if (/[^A-Za-z0-9]/.test(pwd)) score++;

    if (pwd.length < 8 || score <= 2) {
      strengthBar.className = 'h-full w-1/3 bg-red-500 transition-all duration-300 rounded-full';
      strengthText.textContent = 'Weak (at least 8 characters required)';
      strengthText.className = 'text-xs text-red-400 mt-1.5';
    } else if (score <= 3) {
      strengthBar.className = 'h-full w-2/3 bg-amber-500 transition-all duration-300 rounded-full';
      strengthText.textContent = 'Medium (consider increasing length or adding special characters)';
      strengthText.className = 'text-xs text-amber-400 mt-1.5';
    } else {
      strengthBar.className = 'h-full w-full bg-emerald-500 transition-all duration-300 rounded-full';
      strengthText.textContent = 'Strong (good security)';
      strengthText.className = 'text-xs text-emerald-400 mt-1.5';
    }
  }

  passwordInput.addEventListener('input', () => {
    evaluateStrength(passwordInput.value);
  });

  // 切换密码明文/密文显示
  togglePwdBtn.addEventListener('click', () => {
    const isPassword = passwordInput.type === 'password';
    passwordInput.type = isPassword ? 'text' : 'password';
    confirmInput.type = isPassword ? 'text' : 'password';
  });

  // 生成 16 位高强度随机密码并自动复制
  generateBtn.addEventListener('click', async () => {
    const charset = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%^&*_-';
    const array = new Uint32Array(16);
    crypto.getRandomValues(array);
    let generated = '';
    for (let i = 0; i < array.length; i++) {
      generated += charset[array[i] % charset.length];
    }

    passwordInput.value = generated;
    confirmInput.value = generated;
    passwordInput.type = 'text';
    confirmInput.type = 'text';
    evaluateStrength(generated);

    try {
      await navigator.clipboard.writeText(generated);
      showAlert('已生成强密码并复制到剪贴板，请妥善保存！', 'success');
    } catch {
      showAlert('已生成强密码，请手动复制保存！', 'success');
    }
  });

  // 提交初始化表单 -> POST /api/setup
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideAlert();

    const username = usernameInput.value.trim();
    const password = passwordInput.value;
    const confirmPassword = confirmInput.value;

    if (!username) {
      showAlert('Please enter admin username');
      usernameInput.focus();
      return;
    }

    if (password.length < 8) {
      showAlert('For public network security, the password must be at least 8 characters long');
      passwordInput.focus();
      return;
    }

    if (password !== confirmPassword) {
      showAlert('Passwords do not match');
      confirmInput.focus();
      return;
    }

    submitBtn.disabled = true;
    btnText.textContent = 'Now requesting...';
    spinner.classList.remove('hidden');

    try {
      const response = await fetch('/api/setup', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        let errMsg = 'Setup failed, please check the server logs';
        const contentType = response.headers.get('content-type') || '';
        if (contentType.includes('application/json')) {
          const errData = await response.json();
          errMsg = errData.error || errData.message || errMsg;
        } else {
          const text = await response.text();
          if (text) errMsg = text;
        }
        throw new Error(errMsg);
      }

      showAlert('Setup successful! Redirecting to the console...', 'success');
      setTimeout(() => {
        window.location.href = '/';
      }, 800);
    } catch (err) {
      showAlert(err.message);
      submitBtn.disabled = false;
      btnText.textContent = 'Complete Setup and Enter Console';
      spinner.classList.add('hidden');
    }
  });
});