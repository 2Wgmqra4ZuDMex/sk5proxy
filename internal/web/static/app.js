(function () {
  'use strict';

  var api = window.SK5.api;
  var state = window.SK5.state;
  var dom = window.SK5.dom;

  var els = {
    upstreamsList: document.getElementById('upstreams-list'),
    upstreamsEmpty: document.getElementById('upstreams-empty'),
    loading: document.getElementById('loading-state'),
    error: document.getElementById('error-state'),
    errorMessage: document.getElementById('error-message'),
    actionError: document.getElementById('action-error'),
    addUpstreamBtn: document.getElementById('add-upstream-btn'),
    retryBtn: document.getElementById('retry-btn'),
    dialog: document.getElementById('upstream-dialog'),
    form: document.getElementById('upstream-form'),
    dialogTitle: document.getElementById('dialog-title'),
    formError: document.getElementById('form-error'),
    cancelBtn: document.getElementById('cancel-btn'),
    submitBtn: document.getElementById('submit-btn'),
    passwordHint: document.getElementById('password-hint'),
    confirmDialog: document.getElementById('confirm-dialog'),
    confirmCancelBtn: document.getElementById('confirm-cancel-btn'),
    confirmDeleteBtn: document.getElementById('confirm-delete-btn'),
    confirmMessage: document.getElementById('confirm-message'),
    confirmError: document.getElementById('confirm-error'),
    announcer: document.getElementById('status-announcer')
  };

  var editingId = null;
  var deletingId = null;

  function announce(msg) {
    els.announcer.textContent = msg;
    setTimeout(function () { els.announcer.textContent = ''; }, 1000);
  }

  function showView(name) {
    els.loading.hidden = name !== 'loading';
    els.error.hidden = name !== 'error';
    var showContent = name === 'list' || name === 'empty';
    document.getElementById('content-area').hidden = !showContent;
  }

  function clearActionError() {
    els.actionError.hidden = true;
    els.actionError.textContent = '';
  }

  function showActionError(msg) {
    els.actionError.textContent = msg;
    els.actionError.hidden = false;
  }

  function clearFormError() {
    els.formError.hidden = true;
    els.formError.textContent = '';
  }

  function showFormError(msg) {
    els.formError.textContent = msg;
    els.formError.hidden = false;
  }

  function clearConfirmError() {
    els.confirmError.hidden = true;
    els.confirmError.textContent = '';
  }

  function showConfirmError(msg) {
    els.confirmError.textContent = msg;
    els.confirmError.hidden = false;
  }

  function renderUpstreams() {
    var config = state.get();
    els.upstreamsList.textContent = '';
    clearActionError();

    if (config.upstreams.length === 0) {
      els.upstreamsList.hidden = true;
      els.upstreamsEmpty.hidden = false;
      return;
    }
    els.upstreamsList.hidden = false;
    els.upstreamsEmpty.hidden = true;

    var handlers = { edit: openEditDialog, delete: openDeleteConfirm };
    config.upstreams.forEach(function (u) {
      els.upstreamsList.appendChild(dom.buildUpstreamRow(u, handlers));
    });
  }

  function renderAll() {
    renderUpstreams();
    window.SK5.listeners.render();
  }

  async function loadConfig() {
    showView('loading');
    try {
      var config = await api.fetchConfig();
      state.set(config);
      showView('list');
      renderAll();
    } catch (err) {
      els.errorMessage.textContent = err.message;
      showView('error');
    }
  }

  function openAddDialog() {
    editingId = null;
    els.dialogTitle.textContent = '添加上游';
    els.form.reset();
    clearFormError();
    els.passwordHint.hidden = true;
    document.getElementById('field-id').readOnly = false;
    els.dialog.showModal();
  }

  function openEditDialog(id) {
    var u = state.findUpstream(id);
    if (!u) { return; }
    editingId = id;
    els.dialogTitle.textContent = '编辑上游';
    clearFormError();
    els.passwordHint.hidden = false;
    document.getElementById('field-id').value = u.id;
    document.getElementById('field-id').readOnly = true;
    document.getElementById('field-name').value = u.name;
    document.getElementById('field-type').value = u.type;
    document.getElementById('field-address').value = u.address;
    document.getElementById('field-username').value = u.username || '';
    document.getElementById('field-password').value = '';
    els.dialog.showModal();
  }

  function closeDialog() {
    els.dialog.close();
  }

  async function submitForm(event) {
    event.preventDefault();
    clearFormError();

    var fd = new FormData(els.form);
    var payload = {
      id: editingId || fd.get('id'),
      name: fd.get('name'),
      type: fd.get('type'),
      address: fd.get('address'),
      username: fd.get('username'),
      password: fd.get('password')
    };

    els.submitBtn.disabled = true;
    els.submitBtn.textContent = '保存中...';
    try {
      var config = await api.upsertUpstream(payload);
      state.set(config);
      renderAll();
      closeDialog();
      announce(editingId ? '上游已更新' : '上游已添加');
    } catch (err) {
      showFormError(err.message);
    } finally {
      els.submitBtn.disabled = false;
      els.submitBtn.textContent = '保存';
    }
  }

  function openDeleteConfirm(id) {
    var u = state.findUpstream(id);
    if (!u) { return; }
    deletingId = id;
    clearConfirmError();
    els.confirmMessage.textContent = '';
    els.confirmMessage.appendChild(document.createTextNode('确定删除\u201c'));
    var nameNode = document.createElement('span');
    nameNode.className = 'confirm-name';
    nameNode.textContent = u.name;
    els.confirmMessage.appendChild(nameNode);
    els.confirmMessage.appendChild(document.createTextNode('\u201d？删除后无法\u2060恢复。'));
    els.confirmDeleteBtn.disabled = false;
    els.confirmDeleteBtn.textContent = '删除';
    els.confirmDialog.showModal();
  }

  function closeDeleteConfirm() {
    els.confirmDialog.close();
    deletingId = null;
    clearConfirmError();
  }

  async function confirmDelete() {
    if (!deletingId) { return; }
    clearConfirmError();
    els.confirmDeleteBtn.disabled = true;
    els.confirmDeleteBtn.textContent = '删除中...';
    try {
      var config = await api.deleteUpstream(deletingId);
      state.set(config);
      renderAll();
      closeDeleteConfirm();
      announce('上游已删除');
    } catch (err) {
      showConfirmError(err.message);
      els.confirmDeleteBtn.disabled = false;
      els.confirmDeleteBtn.textContent = '删除';
    }
  }

  // --- Event bindings ---
  els.addUpstreamBtn.addEventListener('click', openAddDialog);
  els.cancelBtn.addEventListener('click', closeDialog);
  els.form.addEventListener('submit', submitForm);
  els.retryBtn.addEventListener('click', loadConfig);
  els.confirmCancelBtn.addEventListener('click', closeDeleteConfirm);
  els.confirmDeleteBtn.addEventListener('click', confirmDelete);
  els.dialog.addEventListener('close', function () { editingId = null; });
  els.confirmDialog.addEventListener('close', function () {
    if (deletingId) { deletingId = null; clearConfirmError(); }
  });

  // --- Init listener section ---
  window.SK5.listeners.init({
    list: document.getElementById('listeners-list'),
    empty: document.getElementById('listeners-empty'),
    actionError: document.getElementById('listener-action-error'),
    addBtn: document.getElementById('add-listener-btn'),
    dialog: document.getElementById('listener-dialog'),
    form: document.getElementById('listener-form'),
    dialogTitle: document.getElementById('listener-dialog-title'),
    formError: document.getElementById('listener-form-error'),
    cancelBtn: document.getElementById('listener-cancel-btn'),
    submitBtn: document.getElementById('listener-submit-btn'),
    confirmDialog: document.getElementById('confirm-dialog'),
    confirmCancelBtn: document.getElementById('confirm-cancel-btn'),
    confirmDeleteBtn: document.getElementById('confirm-delete-btn'),
    confirmMessage: document.getElementById('confirm-message'),
    confirmError: document.getElementById('confirm-error')
  });

  // --- Expose for cross-module calls ---
  window.SK5.app = {
    renderAll: renderAll,
    announce: announce
  };

  loadConfig();
})();
