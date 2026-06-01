// Console protection warning against Self-XSS and copy-paste console exploits
console.log(
  "%cSTOP!",
  "color: #eb5757; font-size: 3rem; font-weight: bold; font-family: sans-serif; text-shadow: 0 1px 3px rgba(0,0,0,0.15);"
);
console.log(
  "%cThis is a developer feature. If someone told you to copy-paste something here to 'hack' or retrieve a paste, it is a scam and could compromise your data. Do not execute scripts here unless you understand exactly what they do.",
  "color: #37352f; font-size: 1.1rem; font-family: sans-serif; line-height: 1.4;"
);

// Global variables for creation form
let currentTab = 'text';
let selectedFile = null;

// Tab Switching
function switchTab(tab) {
  currentTab = tab;
  document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
  
  if (tab === 'text') {
    document.getElementById('tab-text').classList.add('active');
    document.getElementById('content-text').classList.add('active');
  } else if (tab === 'file') {
    document.getElementById('tab-file').classList.add('active');
    document.getElementById('content-file').classList.add('active');
  } else if (tab === 'retrieve') {
    document.getElementById('tab-retrieve').classList.add('active');
    document.getElementById('content-retrieve').classList.add('active');
  }

  // Hide settings and main submit button if we are in Retrieve mode
  const settingsGrid = document.querySelector('.settings-grid');
  const submitShare = document.getElementById('submit-share');
  if (tab === 'retrieve') {
    if (settingsGrid) settingsGrid.style.display = 'none';
    if (submitShare) submitShare.style.display = 'none';
  } else {
    if (settingsGrid) settingsGrid.style.display = '';
    if (submitShare) submitShare.style.display = '';
  }
}

// Drag & Drop Event Listeners
const dropZone = document.getElementById('drop-zone');
const fileInput = document.getElementById('file-input');
const fileBox = document.getElementById('file-box');
const selectedFileName = document.getElementById('selected-file-name');
const selectedFileSize = document.getElementById('selected-file-size');

if (dropZone) {
  dropZone.addEventListener('click', () => fileInput.click());

  dropZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    dropZone.classList.add('dragover');
  });

  ['dragleave', 'dragend'].forEach(type => {
    dropZone.addEventListener(type, () => {
      dropZone.classList.remove('dragover');
    });
  });

  dropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropZone.classList.remove('dragover');
    
    if (e.dataTransfer.files.length > 0) {
      handleFileSelect(e.dataTransfer.files[0]);
    }
  });

  fileInput.addEventListener('change', () => {
    if (fileInput.files.length > 0) {
      handleFileSelect(fileInput.files[0]);
    }
  });
}

function handleFileSelect(file) {
  selectedFile = file;
  selectedFileName.textContent = file.name;
  selectedFileSize.textContent = formatBytes(file.size);
  
  dropZone.style.display = 'none';
  fileBox.style.display = 'flex';
}

function clearFile() {
  selectedFile = null;
  fileInput.value = '';
  dropZone.style.display = 'block';
  fileBox.style.display = 'none';
}

function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// Submit Upload Action
const submitBtn = document.getElementById('submit-share');
if (submitBtn) {
  submitBtn.addEventListener('click', async () => {
    const ttl = document.getElementById('expiry-select').value;
    const burnOnRead = document.getElementById('burn-toggle').checked;
    
    const formData = new FormData();
    formData.append('ttl', ttl);
    formData.append('burn', burnOnRead);

    if (currentTab === 'text') {
      const text = document.getElementById('paste-text').value.trim();
      if (!text) {
        alert('Please enter some text paste content!');
        return;
      }
      formData.append('content', text);
    } else {
      if (!selectedFile) {
        alert('Please select a file to upload!');
        return;
      }
      formData.append('file', selectedFile);
    }

    // Visual button loading state
    submitBtn.disabled = true;
    submitBtn.textContent = 'Generating Share...';

    try {
      const response = await fetch('/api/upload', {
        method: 'POST',
        body: formData
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || 'Upload failed');
      }

      const result = await response.json();
      showSuccess(result);
    } catch (err) {
      alert('Error: ' + err.message);
      submitBtn.disabled = false;
      submitBtn.textContent = 'Generate Ephemeral Share';
    }
  });
}

// Render Success Screen
function showSuccess(data) {
  document.getElementById('form-card').style.display = 'none';
  const successCard = document.getElementById('success-card');
  successCard.style.display = 'block';

  // Construct absolute URL
  const shareUrl = window.location.origin + '/p/' + data.id;
  document.getElementById('result-url').textContent = shareUrl;
  
  // Format expiry display
  const durationMap = {
    '5m': '5 Minutes',
    '1h': '1 Hour',
    '4h': '4 Hours',
    '1d': '1 Day',
    '7d': '7 Days'
  };
  const selectedExpiry = document.getElementById('expiry-select').value;
  document.getElementById('result-expiry').textContent = durationMap[selectedExpiry] || selectedExpiry;
  document.getElementById('result-burn').textContent = data.burn_on_read ? '🔥 Active (Burns on Read)' : 'Disabled';
  document.getElementById('result-delete-token').textContent = data.delete_token;

  // Configure copy button
  setupCopyBtn(document.getElementById('copy-btn'), shareUrl);
  
  // Configure delete now button
  const deleteBtn = document.getElementById('delete-btn');
  deleteBtn.onclick = async () => {
    if (confirm('Are you sure you want to delete this share permanently right now?')) {
      try {
        const res = await fetch(`/p/${data.id}?token=${data.delete_token}`, {
          method: 'DELETE'
        });
        if (res.ok) {
          alert('Share deleted successfully!');
          resetApp();
        } else {
          const errMsg = await res.text();
          alert('Delete failed: ' + errMsg);
        }
      } catch (e) {
        alert('Delete failed: ' + e);
      }
    }
  };
}

function setupCopyBtn(btn, text) {
  btn.onclick = async () => {
    try {
      await navigator.clipboard.writeText(text);
      const originalText = btn.textContent;
      btn.textContent = '✓ Copied!';
      btn.style.filter = 'hue-rotate(90deg)';
      setTimeout(() => {
        btn.textContent = originalText;
        btn.style.filter = 'none';
      }, 2000);
    } catch (err) {
      console.error('Failed to copy: ', err);
    }
  };
}

function resetApp() {
  document.getElementById('form-card').style.display = 'block';
  document.getElementById('success-card').style.display = 'none';
  document.getElementById('paste-text').value = '';
  
  const retrieveInput = document.getElementById('retrieve-code');
  if (retrieveInput) retrieveInput.value = '';
  
  clearFile();
  switchTab('text');
  if (submitBtn) {
    submitBtn.disabled = false;
    submitBtn.textContent = 'Generate Ephemeral Share';
  }
}

// Paste View Code (Self-Destruct Countdown)
const timeLeftEl = document.getElementById('time-left');
if (timeLeftEl) {
  const expiryTimestamp = parseInt(timeLeftEl.dataset.expiry, 10);
  
  function updateCountdown() {
    const now = Math.floor(Date.now() / 1000);
    const diff = expiryTimestamp - now;

    if (diff <= 0) {
      timeLeftEl.textContent = 'Expired';
      timeLeftEl.style.color = 'var(--accent)';
      setTimeout(() => {
        window.location.reload();
      }, 1000);
      return;
    }

    const d = Math.floor(diff / 86400);
    const h = Math.floor((diff % 86400) / 3600);
    const m = Math.floor((diff % 3600) / 60);
    const s = diff % 60;

    let timeStr = 'Expires in: ';
    if (d > 0) timeStr += `${d}d `;
    if (h > 0 || d > 0) timeStr += `${h}h `;
    if (m > 0 || h > 0 || d > 0) timeStr += `${m}m `;
    timeStr += `${s}s`;

    timeLeftEl.textContent = timeStr;
  }

  updateCountdown();
  setInterval(updateCountdown, 1000);
}

// View copy text button
const copyViewBtn = document.getElementById('copy-view-btn');
if (copyViewBtn) {
  setupCopyBtn(copyViewBtn, copyViewBtn.dataset.clipboard);
}

// CLI Client Modal management
const cliBtn = document.getElementById('cli-btn');
const cliModal = document.getElementById('cli-modal');
const closeModalBtn = document.getElementById('close-modal-btn');
const modalOkBtn = document.getElementById('modal-ok-btn');

if (cliBtn && cliModal) {
  cliBtn.onclick = () => cliModal.classList.add('active');
  
  const closeModal = () => cliModal.classList.remove('active');
  closeModalBtn.onclick = closeModal;
  modalOkBtn.onclick = closeModal;
  
  cliModal.onclick = (e) => {
    if (e.target === cliModal) closeModal();
  };
}

// Retrieve share action
const retrieveBtn = document.getElementById('btn-retrieve-action');
if (retrieveBtn) {
  retrieveBtn.addEventListener('click', () => {
    let input = document.getElementById('retrieve-code').value.trim();
    if (!input) {
      alert('Please enter a share code or link!');
      return;
    }
    
    // Extract ID (either 8 alphanumeric chars, or parse from full URL)
    let code = input;
    if (input.includes('/p/')) {
      const parts = input.split('/p/');
      code = parts[parts.length - 1].split('?')[0].split('#')[0].trim();
    } else if (input.includes('/raw/')) {
      const parts = input.split('/raw/');
      code = parts[parts.length - 1].split('?')[0].split('#')[0].trim();
    }
    
    // Strip trailing slashes or special characters
    code = code.replace(/[^a-zA-Z0-9]/g, '');
    
    if (code.length === 0) {
      alert('Could not resolve a valid share code from your input.');
      return;
    }
    
    // Redirect to the share page
    window.location.href = '/p/' + code;
  });
}
