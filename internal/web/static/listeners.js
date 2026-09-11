(function () {
  'use strict';

  var api = window.SK5.api;
  var state = window.SK5.state;
  var dom = window.SK5.dom;

  var els = {};
  var editingId = null;
  var switchingId = null;
  var confirmTarget = null; // { type: 'listener', id: string, name: string }

  function init(elements) {
    els = elements;
    els.addBtn.addEventListener('click', openAddDialog);
    els.cancelBtn.addEventListener('click', function () { els.dialog.close(); });
    els.form.addEventListener('submit', submitForm);
    els.dialog.addEventListener('close', function () { editingId = null; });
    els.confirmCancelBtn.addEventListener('click', closeConfirm);
    els.confirmDeleteBtn.addEventListener('click', confirmDelete);
    els.confirmDialog.addEventListener('close', function () { confirmTarget = null; clearConfirmError(); });
    render();
  }

  function render() {
    var config = state.get();
    els.list.textContent = '';
    clearActionError();

    if (config.listeners.length === 0) {
      els.list.hidden = true;
      els.empty.hidden = false;
      return;
    }
    els.list.hidden = false;
    els.empty.hidden = true;

    var handlers = {
      switchUpstream: switchUpstream,
      toggle: toggleListener,
      edit: openEditDialog,
      delete: openDeleteConfirm
    };

    config.listeners.forEach(function (l) {
      var switching = l.id === switchingId;
      els.list.appendChild(dom.buildListenerRow(l, switching, handlers));
    });
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

  function openAddDialog() {
    editingId = null;
    els.dialogTitle.textContent = '添加监听';
    els.form.reset();
    clearFormError();
    document.getElementById('listener-field-id').readOnly = false;
    document.getElementById('listener-field-id').closest('.form-field').hidden = false;
    document.getElementById('listener-field-enabled').checked = true;
    populateUpstreamSelect('');
    els.dialog.showModal();
  }

  function openEditDialog(id) {
    var l = state.findListener(id);
    if (!l) { return; }
    editingId = id;
    els.dialogTitle.textContent = '编辑监听';
    clearFormError();
    document.getElementById('listener-field-id').value = l.id;
    document.getElementById('listener-field-id').readOnly = true;
    document.getElementById('listener-field-id').closest('.form-field').hidden = true;
    document.getElementById('listener-field-name').value = l.name;
    document.getElementById('listener-field-type').value = l.type;
    document.getElementById('listener-field-address').value = l.address;
    document.getElementById('listener-field-enabled').checked = l.enabled;
    populateUpstreamSelect(l.upstreamId);
    els.dialog.showModal();
  }

  function populateUpstreamSelect(selectedId) {
    var select = document.getElementById('listener-field-upstream');
    select.textContent = '';
    var emptyOpt = document.createElement('option');
    emptyOpt.value = '';
    emptyOpt.textContent = '未选择';
    select.appendChild(emptyOpt);
    var upstreams = state.get().upstreams;
    for (var i = 0; i < upstreams.length; i++) {
      var opt = document.createElement('option');
      opt.value = upstreams[i].id;
      opt.textContent = upstreams[i].name + ' (' + upstreams[i].type.toUpperCase() + ')';
      select.appendChild(opt);
    }
    select.value = selectedId || '';
  }

  async function submitForm(event) {
    event.preventDefault();
    clearFormError();

    var fd = new FormData(els.form);
    var id = editingId || fd.get('id');
    var payload = {
      name: fd.get('name'),
      type: fd.get('type'),
      address: fd.get('address'),
      upstreamId: fd.get('upstream'),
      enabled: document.getElementById('listener-field-enabled').checked
    };

    els.submitBtn.disabled = true;
    els.submitBtn.textContent = '保存中...';
    try {
      var config = await api.upsertListener(id, payload);
      state.set(config);
      window.SK5.app.renderAll();
      els.dialog.close();
      window.SK5.app.announce(editingId ? '监听已更新' : '监听已添加');
    } catch (err) {
      showFormError(err.message);
    } finally {
      els.submitBtn.disabled = false;
      els.submitBtn.textContent = '保存';
    }
  }

  async function switchUpstream(id, upstreamId) {
    clearActionError();
    switchingId = id;
    render();
    try {
      var config = await api.switchListener(id, upstreamId);
      state.set(config);
      switchingId = null;
      window.SK5.app.renderAll();
      window.SK5.app.announce('已切换上游');
    } catch (err) {
      switchingId = null;
      render();
      showActionError(err.message);
    }
  }

  async function toggleListener(id, enabled) {
    clearActionError();
    var l = state.findListener(id);
    if (!l) { return; }
    try {
      var config = await api.upsertListener(id, {
        name: l.name, type: l.type, address: l.address,
        upstreamId: l.upstreamId, enabled: enabled
      });
      state.set(config);
      window.SK5.app.renderAll();
      window.SK5.app.announce(enabled ? '已启用监听' : '已禁用监听');
    } catch (err) {
      showActionError(err.message);
    }
  }

  function openDeleteConfirm(id) {
    var l = state.findListener(id);
    if (!l) { return; }
    confirmTarget = { type: 'listener', id: id, name: l.name };
    clearConfirmError();
    els.confirmMessage.textContent = '';
    els.confirmMessage.appendChild(document.createTextNode('确定删除\u201c'));
    var nameNode = document.createElement('span');
    nameNode.className = 'confirm-name';
    nameNode.textContent = l.name;
    els.confirmMessage.appendChild(nameNode);
    els.confirmMessage.appendChild(document.createTextNode('\u201d？删除后无法\u2060恢复。'));
    els.confirmDeleteBtn.disabled = false;
    els.confirmDeleteBtn.textContent = '删除';
    els.confirmDialog.showModal();
  }

  function closeConfirm() {
    els.confirmDialog.close();
    confirmTarget = null;
    clearConfirmError();
  }

  async function confirmDelete() {
    if (!confirmTarget) { return; }
    clearConfirmError();
    els.confirmDeleteBtn.disabled = true;
    els.confirmDeleteBtn.textContent = '删除中...';
    try {
      var config = await api.deleteListener(confirmTarget.id);
      state.set(config);
      window.SK5.app.renderAll();
      closeConfirm();
      window.SK5.app.announce('监听已删除');
    } catch (err) {
      showConfirmError(err.message);
      els.confirmDeleteBtn.disabled = false;
      els.confirmDeleteBtn.textContent = '删除';
    }
  }

  window.SK5 = window.SK5 || {};
  window.SK5.listeners = {
    init: init,
    render: render
  };
})();
