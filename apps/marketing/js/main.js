/* ========== 摊伴营销网站 ========== */

// API 基础地址（生产环境走同域，开发环境可修改）
const API_BASE = window.location.hostname === 'localhost'
  ? 'https://tbapi.666qwe.cn/api/v1'
  : '/api/v1';

// ========== 导航栏 ==========
const header = document.getElementById('header');
const menuToggle = document.getElementById('menuToggle');
const nav = document.getElementById('nav');
const backTop = document.getElementById('backTop');

window.addEventListener('scroll', () => {
  header.classList.toggle('scrolled', window.scrollY > 10);
  backTop.classList.toggle('visible', window.scrollY > 600);
});

menuToggle.addEventListener('click', () => {
  nav.classList.toggle('open');
  menuToggle.classList.toggle('active');
});

// 移动端点击导航链接后关闭菜单
nav.querySelectorAll('.nav-link').forEach(link => {
  link.addEventListener('click', () => {
    nav.classList.remove('open');
    menuToggle.classList.remove('active');
  });
});

backTop.addEventListener('click', () => {
  window.scrollTo({ top: 0, behavior: 'smooth' });
});

// ========== 数学验证码 ==========
let captchaAnswer = 0;

function generateCaptcha() {
  const a = Math.floor(Math.random() * 10) + 1;
  const b = Math.floor(Math.random() * 10) + 1;
  const ops = ['+', '-'];
  const op = ops[Math.floor(Math.random() * ops.length)];
  captchaAnswer = op === '+' ? a + b : a - b;
  document.getElementById('captchaQuestion').textContent = `${a} ${op} ${b} = ?`;
}

document.getElementById('captchaBox').addEventListener('click', generateCaptcha);
generateCaptcha();

// ========== 网站配置加载 ==========
async function loadSiteSettings() {
  try {
    const res = await fetch(`${API_BASE}/public/website-settings`);
    if (!res.ok) return;
    const json = await res.json();
    const settings = json.data || json;

    // 客服电话
    if (settings.contactPhone) {
      document.getElementById('contactPhone').textContent = `📞 ${settings.contactPhone}`;
    }
    // 客服微信
    if (settings.contactWechat) {
      document.getElementById('contactWechat').textContent = `💬 ${settings.contactWechat}`;
    }
    // 客服邮箱
    if (settings.contactEmail) {
      document.getElementById('contactEmail').textContent = `📧 ${settings.contactEmail}`;
    }
    // 客服微信二维码
    if (settings.wechatQrUrl) {
      const qrBox = document.getElementById('contactQr');
      qrBox.innerHTML = `<img src="${settings.wechatQrUrl}" alt="客服微信二维码">`;
    }
    // Hero 背景图
    if (settings.heroImageUrl) {
      const mockups = document.getElementById('heroMockups');
      mockups.style.background = `url(${settings.heroImageUrl}) center/contain no-repeat`;
    }
  } catch (e) {
    // 静默失败，使用默认内容
    console.log('网站配置加载失败，使用默认配置');
  }
}
loadSiteSettings();

// ========== 客户登记表单 ==========
const registerForm = document.getElementById('registerForm');
const submitBtn = document.getElementById('submitBtn');
const formTip = document.getElementById('formTip');

registerForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  formTip.className = 'form-tip';
  formTip.textContent = '';

  const name = document.getElementById('leadName').value.trim();
  const phone = document.getElementById('leadPhone').value.trim();
  const captcha = document.getElementById('leadCaptcha').value.trim();
  const honeypot = document.getElementById('leadWebsite').value;

  // 前端验证
  if (!name) { showError('请输入您的姓名'); return; }
  if (!/^1\d{10}$/.test(phone)) { showError('请输入正确的手机号码'); return; }
  if (!captcha) { showError('请输入验证码'); return; }
  if (parseInt(captcha) !== captchaAnswer) { showError('验证码不正确，请重新输入'); generateCaptcha(); return; }

  // Honeypot 检测（机器人会填这个字段）
  if (honeypot) { showError('提交失败，请刷新页面重试'); return; }

  submitBtn.disabled = true;
  submitBtn.textContent = '提交中...';

  try {
    const res = await fetch(`${API_BASE}/public/leads`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, phone, source: 'website' }),
    });

    const json = await res.json();

    if (res.ok) {
      formTip.className = 'form-tip success';
      formTip.textContent = '提交成功！我们会在 1 个工作日内联系您。';
      registerForm.reset();
      generateCaptcha();
    } else {
      const msg = json?.error?.message || json?.message || '提交失败，请稍后重试';
      showError(msg);
    }
  } catch (err) {
    showError('网络连接失败，请检查网络后重试');
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = '申请免费体验';
  }
});

function showError(msg) {
  formTip.className = 'form-tip error';
  formTip.textContent = msg;
}
