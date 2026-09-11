(function () {
  'use strict';

  var currentConfig = { activeId: '', upstreams: [], listeners: [] };

  function set(config) {
    currentConfig = {
      activeId: config.activeId || '',
      upstreams: Array.isArray(config.upstreams) ? config.upstreams.slice() : [],
      listeners: Array.isArray(config.listeners) ? config.listeners.slice() : []
    };
  }

  function get() {
    return currentConfig;
  }

  function findUpstream(id) {
    for (var i = 0; i < currentConfig.upstreams.length; i++) {
      if (currentConfig.upstreams[i].id === id) {
        return currentConfig.upstreams[i];
      }
    }
    return null;
  }

  function findListener(id) {
    for (var i = 0; i < currentConfig.listeners.length; i++) {
      if (currentConfig.listeners[i].id === id) {
        return currentConfig.listeners[i];
      }
    }
    return null;
  }

  function upstreamName(id) {
    var u = findUpstream(id);
    return u ? u.name : '';
  }

  window.SK5 = window.SK5 || {};
  window.SK5.state = {
    get: get,
    set: set,
    findUpstream: findUpstream,
    findListener: findListener,
    upstreamName: upstreamName
  };
})();
