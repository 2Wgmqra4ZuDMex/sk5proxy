(function () {
  'use strict';

  var API_BASE = '/api/config';

  var ERROR_MAP = {
    'invalid upstream configuration': '上游配置无效',
    'invalid upstream ID': '标识符格式无效',
    'upstream not found': '未找到该上游',
    'cannot delete active upstream': '请先停用该上游再删除',
    'cannot delete referenced upstream': '该上游被监听器引用，请先删除相关监听器',
    'type must be socks5 or http': '类型必须为 SOCKS5 或 HTTP',
    'invalid JSON': '请求数据格式无效',
    'save failed': '保存失败，请重试',
    'prepare selector failed': '配置生效失败，请重试',
    'listener not found': '未找到该监听器',
    'listener bind failed': '监听器绑定失败，请检查地址是否被占用',
    'bind failed': '端口绑定失败，请检查地址是否被占用',
    'duplicate enabled listener address': '该地址已被其他启用的监听器使用'
  };

  var FALLBACK_ERROR = '操作失败，请检查配置';

  function mapError(raw) {
    if (!raw || typeof raw !== 'string') { return FALLBACK_ERROR; }
    for (var key in ERROR_MAP) {
      if (raw.indexOf(key) !== -1) { return ERROR_MAP[key]; }
    }
    return FALLBACK_ERROR;
  }

  async function request(method, path, body) {
    var opts = { method: method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    var res;
    try {
      res = await fetch(API_BASE + path, opts);
    } catch (err) {
      throw new Error('网络连接失败，请检查网络');
    }
    var data;
    var isJson = (res.headers.get('content-type') || '').includes('application/json');
    if (isJson) {
      try {
        data = await res.json();
      } catch (parseErr) {
        // Malformed JSON — leave data undefined so fallback error is used
      }
    }
    if (!res.ok) {
      var raw = (data && data.error) ? data.error : null;
      throw new Error(raw ? mapError(raw) : FALLBACK_ERROR);
    }
    return data;
  }

  window.SK5 = window.SK5 || {};
  window.SK5.api = {
    fetchConfig: function () { return request('GET', ''); },
    upsertUpstream: function (payload) { return request('PUT', '/upstreams', payload); },
    deleteUpstream: function (id) { return request('DELETE', '/upstreams/' + encodeURIComponent(id)); },
    upsertListener: function (id, payload) { return request('PUT', '/listeners/' + encodeURIComponent(id), payload); },
    switchListener: function (id, upstreamId) { return request('POST', '/listeners/' + encodeURIComponent(id) + '/switch', { upstreamId: upstreamId }); },
    deleteListener: function (id) { return request('DELETE', '/listeners/' + encodeURIComponent(id)); }
  };
})();
