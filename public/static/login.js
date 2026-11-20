document.addEventListener('DOMContentLoaded', () => {
  const form = document.getElementById('login-form');
  const usernameInput = document.getElementById('username');
  const passwordInput = document.getElementById('password');
  const togglePwdBtn = document.getElementById('toggle-pwd');
  const alertBox = document.getElementById('alert-box');
  const submitBtn = document.getElementById('submit-btn');
  const btnText = submitBtn.querySelector('.btn-text');
  const spinner = submitBtn.querySelector('.spinner');

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

  // 切换密码明文/密文显示
  togglePwdBtn.addEventListener('click', () => {
    passwordInput.type = passwordInput.type === 'password' ? 'text' : 'password';
  });

  // 提交登录表单 -> POST /api/login
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideAlert();

    const username = usernameInput.value.trim();
    const password = passwordInput.value;

    if (!username) {
      showAlert('请输入用户名');
      usernameInput.focus();
      return;
    }

    if (!password) {
      showAlert('请输入访问密码');
      passwordInput.focus();
      return;
    }

    submitBtn.disabled = true;
    btnText.textContent = '正在验证...';
    spinner.classList.remove('hidden');

    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        let errMsg = '用户名或密码错误';
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

      showAlert('登录成功，正在进入控制台...', 'success');

      // 支持登录后跳回原访问路径（仅限同源相对路径，防止开放重定向漏洞）
      const params = new URLSearchParams(window.location.search);
      const redirect = params.get('redirect');
      const targetUrl = (redirect && redirect.startsWith('/') && !redirect.startsWith('//')) ? redirect : '/';

      setTimeout(() => {
        window.location.href = targetUrl;
      }, 400);
    } catch (err) {
      showAlert(err.message);
      passwordInput.value = '';
      passwordInput.focus();
      submitBtn.disabled = false;
      btnText.textContent = '登录';
      spinner.classList.add('hidden');
    }
  });
});