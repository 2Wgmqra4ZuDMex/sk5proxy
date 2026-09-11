(function () {
  'use strict';

  var state = window.SK5.state;

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) { node.className = className; }
    if (text !== undefined) { node.textContent = text; }
    return node;
  }

  // --- Upstream Row (no activate, no active badge) ---

  function buildUpstreamRow(upstream, handlers) {
    var row = el('div', 'upstream-row');
    row.setAttribute('role', 'listitem');

    var header = el('div', 'upstream-header');
    header.appendChild(el('div', 'upstream-name', upstream.name));
    var badges = el('div', 'upstream-badges');
    badges.appendChild(el('span', 'badge badge-type', upstream.type.toUpperCase()));
    header.appendChild(badges);
    row.appendChild(header);

    var details = el('div', 'upstream-details');
    var addrRow = el('div', 'upstream-detail');
    addrRow.appendChild(el('span', 'upstream-detail-label', '地址:'));
    addrRow.appendChild(el('span', undefined, upstream.address));
    details.appendChild(addrRow);
    if (upstream.username) {
      var userRow = el('div', 'upstream-detail');
      userRow.appendChild(el('span', 'upstream-detail-label', '用户:'));
      userRow.appendChild(el('span', undefined, upstream.username));
      details.appendChild(userRow);
    }
    row.appendChild(details);

    var actions = el('div', 'upstream-actions');
    var editBtn = el('button', 'btn btn-secondary', '编辑');
    editBtn.addEventListener('click', function () { handlers.edit(upstream.id); });
    actions.appendChild(editBtn);
    var deleteBtn = el('button', 'btn btn-destructive', '删除');
    deleteBtn.addEventListener('click', function () { handlers.delete(upstream.id); });
    actions.appendChild(deleteBtn);
    row.appendChild(actions);

    return row;
  }

  // --- Listener Row ---

  function buildListenerRow(listener, switching, handlers) {
    var row = el('div', 'listener-row' + (listener.enabled ? ' enabled' : ' disabled'));
    row.setAttribute('role', 'listitem');

    var header = el('div', 'listener-header');
    header.appendChild(el('div', 'listener-name', listener.name));
    var badges = el('div', 'listener-badges');
    badges.appendChild(el('span', 'badge badge-type', listener.type.toUpperCase()));
    var enabledBadge = el('span', 'badge ' + (listener.enabled ? 'badge-enabled' : 'badge-disabled'), listener.enabled ? '启用' : '禁用');
    badges.appendChild(enabledBadge);
    header.appendChild(badges);
    row.appendChild(header);

    var details = el('div', 'listener-details');
    var addrRow = el('div', 'listener-detail');
    addrRow.appendChild(el('span', 'listener-detail-label', '地址:'));
    addrRow.appendChild(el('span', undefined, listener.address));
    details.appendChild(addrRow);
    var upRow = el('div', 'listener-detail');
    upRow.appendChild(el('span', 'listener-detail-label', '上游:'));
    upRow.appendChild(el('span', undefined, state.upstreamName(listener.upstreamId) || '未选择'));
    details.appendChild(upRow);
    row.appendChild(details);

    // Switch control: upstream select + apply button
    var switchBar = el('div', 'listener-switch');
    var select = document.createElement('select');
    select.className = 'input listener-upstream-select';
    select.setAttribute('aria-label', '选择上游');
    var emptyOpt = document.createElement('option');
    emptyOpt.value = '';
    emptyOpt.textContent = '未选择';
    select.appendChild(emptyOpt);
    var upstreams = state.get().upstreams;
    for (var i = 0; i < upstreams.length; i++) {
      var opt = document.createElement('option');
      opt.value = upstreams[i].id;
      opt.textContent = upstreams[i].name + ' (' + upstreams[i].type.toUpperCase() + ')';
      if (upstreams[i].id === listener.upstreamId) { opt.selected = true; }
      select.appendChild(opt);
    }
    select.value = listener.upstreamId || '';
    switchBar.appendChild(select);

    var switchBtn = el('button', 'btn btn-primary btn-sm', switching ? '切换中...' : '切换');
    if (switching) { switchBtn.disabled = true; }
    switchBtn.addEventListener('click', function () {
      handlers.switchUpstream(listener.id, select.value);
    });
    switchBar.appendChild(switchBtn);
    row.appendChild(switchBar);

    // Actions
    var actions = el('div', 'listener-actions');
    var toggleBtn = el('button', 'btn btn-secondary', listener.enabled ? '禁用' : '启用');
    toggleBtn.addEventListener('click', function () { handlers.toggle(listener.id, !listener.enabled); });
    actions.appendChild(toggleBtn);
    var editBtn = el('button', 'btn btn-secondary', '编辑');
    editBtn.addEventListener('click', function () { handlers.edit(listener.id); });
    actions.appendChild(editBtn);
    var deleteBtn = el('button', 'btn btn-destructive', '删除');
    deleteBtn.addEventListener('click', function () { handlers.delete(listener.id); });
    actions.appendChild(deleteBtn);
    row.appendChild(actions);

    return row;
  }

  window.SK5 = window.SK5 || {};
  window.SK5.dom = {
    buildUpstreamRow: buildUpstreamRow,
    buildListenerRow: buildListenerRow,
    el: el
  };
})();
