// WebView.Mobile's script, run in every frame before the page's own. It is an expression called with what navigator.platform reports and whether to emulate a touch screen, as WebKit's desktop backends (macOS, Linux) have it do: the page then sees one without hover, and the mouse's drags reach it as a finger's touches. Either way a wheel or trackpad scroll that nothing under it would scroll reaches the page as a finger's swipe, which WebView2's own emulation (Windows) leaves a wheel.
(function (platform, emulate) {
  'use strict';
  // Once per frame: run twice, it would double every touch.
  var mark = Symbol.for('nsgo.webMobile');
  if (window[mark]) return;
  Object.defineProperty(window, mark, { value: true });
  var slop = 6; // CSS pixels a press moves before it is a drag
  var drag = null, swallow = false, wheel = null, glide = null, ids = 0;

  // WebKit on macOS has no Touch or TouchEvent: events of these classes carry the same fields.
  var native = false;
  try {
    native = new window.TouchEvent('touchstart', { touches: [new window.Touch({ identifier: 0, target: document })] }).touches.length === 1;
  } catch (e) { native = false; }
  var Touch = window.Touch, TouchEvent = window.TouchEvent;
  if (!native) {
    var TouchList = function (list) {
      for (var i = 0; i < list.length; i++) this[i] = list[i];
      Object.defineProperty(this, 'length', { value: list.length });
    };
    TouchList.prototype.item = function (i) { return this[i] || null; };
    TouchList.prototype[Symbol.iterator] = Array.prototype[Symbol.iterator];
    Touch = function Touch(init) {
      var self = this;
      ['identifier', 'target', 'clientX', 'clientY', 'screenX', 'screenY', 'pageX', 'pageY', 'radiusX', 'radiusY', 'rotationAngle', 'force'].forEach(function (k) {
        Object.defineProperty(self, k, { value: init[k] === undefined ? 0 : init[k], enumerable: true });
      });
    };
    TouchEvent = class TouchEvent extends UIEvent {
      constructor(type, init) {
        init = init || {};
        super(type, init);
        var self = this;
        ['touches', 'targetTouches', 'changedTouches'].forEach(function (k) {
          Object.defineProperty(self, k, { value: new TouchList(init[k] || []), enumerable: true });
        });
        ['altKey', 'metaKey', 'ctrlKey', 'shiftKey'].forEach(function (k) {
          Object.defineProperty(self, k, { value: !!init[k], enumerable: true });
        });
      }
    };
    if (emulate) {
      [['Touch', Touch], ['TouchEvent', TouchEvent], ['TouchList', TouchList]].forEach(function (c) {
        if (typeof window[c[0]] !== 'function') Object.defineProperty(window, c[0], { value: c[1], writable: true, configurable: true });
      });
    }
  }

  var finger = function (id, target, x, y, sx, sy) {
    return new Touch({ identifier: id, target: target, clientX: x, clientY: y, screenX: sx, screenY: sy,
      pageX: x + scrollX, pageY: y + scrollY, radiusX: 11.5, radiusY: 11.5, rotationAngle: 0, force: 1 });
  };
  // fire dispatches a touch event of one finger t at target, with no fingers down for an end, and reports whether the page prevented its default.
  var fire = function (kind, target, t, e) {
    var down = kind === 'touchstart' || kind === 'touchmove' ? [t] : [];
    var ev = new TouchEvent(kind, { bubbles: true, cancelable: kind !== 'touchcancel', composed: true, view: window,
      touches: down, targetTouches: down, changedTouches: [t],
      altKey: !!(e && e.altKey), metaKey: !!(e && e.metaKey), ctrlKey: !!(e && e.ctrlKey), shiftKey: !!(e && e.shiftKey) });
    return !target.dispatchEvent(ev);
  };
  var targetOf = function (e) {
    var path = e.composedPath && e.composedPath();
    return path && path.length ? path[0] : e.target;
  };

  // What a finger would scroll: the nearest element that scrolls along the axis, or the page, and in delta's direction unless it is 0.
  var parentOf = function (el) { return el.parentElement || (el.parentNode && el.parentNode.host) || null; };
  var canScroll = function (el, vertical, delta) {
    var at = vertical ? el.scrollTop : el.scrollLeft;
    var max = vertical ? el.scrollHeight - el.clientHeight : el.scrollWidth - el.clientWidth;
    return max > 0 && (delta === 0 || (delta > 0 ? at < max - 0.5 : at > 0.5));
  };
  var scrollerOf = function (el, vertical, delta) {
    var root = document.scrollingElement || document.documentElement;
    for (; el && el.nodeType === 1 && el !== root && el !== document.body; el = parentOf(el)) {
      var s = getComputedStyle(el), overflow = vertical ? s.overflowY : s.overflowX;
      if ((overflow === 'auto' || overflow === 'scroll') && canScroll(el, vertical, delta)) return el;
    }
    // The page scrolls unless its root or body hides its overflow, as pages do under a dialog.
    var hidden = [root, document.body].some(function (el) {
      var overflow = el && (vertical ? getComputedStyle(el).overflowY : getComputedStyle(el).overflowX);
      return overflow === 'hidden' || overflow === 'clip';
    });
    return root && !hidden && canScroll(root, vertical, delta) ? root : null;
  };
  var stopGlide = function () {
    if (glide) cancelAnimationFrame(glide.frame);
    glide = null;
  };

  if (emulate) {
    var define = function (object, name, get) {
      Object.defineProperty(object, name, { get: get, configurable: true, enumerable: true });
    };
    define(Navigator.prototype, 'maxTouchPoints', function () { return 5; });
    if (platform) define(Navigator.prototype, 'platform', function () { return platform; });
    if (!('orientation' in window)) define(window, 'orientation', function () { return innerWidth > innerHeight ? 90 : 0; });

    // (pointer: coarse) and (hover: none) match in matchMedia; stylesheets keep the device's.
    var matchMedia = window.matchMedia;
    var always = '(min-width: 0px)', never = '((max-width: 0px) and (min-width: 1px))';
    var feature = /\(\s*(any-)?(pointer|hover)\s*(?::\s*([a-z-]+)\s*)?\)/gi;
    window.matchMedia = function (query) {
      return matchMedia.call(window, String(query).replace(feature, function (_, any, name, value) {
        value = (value || '').toLowerCase();
        var match = name.toLowerCase() === 'pointer' ? value === '' || value === 'coarse' : value === 'none';
        return match ? always : never;
      }));
    };

    if (!('ontouchstart' in window)) {
      [Window.prototype, Document.prototype, HTMLElement.prototype, SVGElement.prototype].forEach(function (proto) {
        ['touchstart', 'touchmove', 'touchend', 'touchcancel'].forEach(function (kind) {
          var handlers = new WeakMap();
          Object.defineProperty(proto, 'on' + kind, {
            configurable: true, enumerable: true,
            get: function () { var h = handlers.get(this); return h ? h.fn : null; },
            set: function (fn) {
              var old = handlers.get(this);
              if (old) this.removeEventListener(kind, old.listener);
              handlers.delete(this);
              if (typeof fn !== 'function') return;
              var h = { fn: fn, listener: function (e) { if (h.fn.call(this, e) === false) e.preventDefault(); } };
              handlers.set(this, h);
              this.addEventListener(kind, h.listener);
            }
          });
        });
      });
    }

    // A drag the page leaves alone pans what it is over, as a finger does, and glides on when let go. Snapping waits for its end.
    var pannable = function (el, vertical) {
      for (; el && el.nodeType === 1; el = parentOf(el)) {
        var action = getComputedStyle(el).touchAction;
        if (action === 'none' || action.indexOf(vertical ? 'pan-x' : 'pan-y') >= 0 && action.indexOf(vertical ? 'pan-y' : 'pan-x') < 0) return false;
      }
      return true;
    };
    var scrollBy = function (el, vertical, by) {
      var before = vertical ? el.scrollTop : el.scrollLeft;
      if (vertical) el.scrollTop = before + by; else el.scrollLeft = before + by;
      return (vertical ? el.scrollTop : el.scrollLeft) !== before;
    };
    var unsnap = function (pan) {
      if (getComputedStyle(pan.el).scrollSnapType.indexOf('none') === 0) return;
      pan.snap = pan.el.style.getPropertyValue('scroll-snap-type');
      pan.el.style.setProperty('scroll-snap-type', 'none', 'important');
    };
    var fling = function (pan, velocity) {
      if (pan.snap !== undefined) {
        pan.el.style.removeProperty('scroll-snap-type');
        if (pan.snap) pan.el.style.setProperty('scroll-snap-type', pan.snap);
        var by = Math.abs(velocity) > 0.3 ? (velocity > 0 ? 1 : -1) : 0;
        pan.el.scrollBy({ left: pan.vertical ? 0 : by, top: pan.vertical ? by : 0, behavior: 'smooth' });
        return;
      }
      if (Math.abs(velocity) < 0.1) return;
      var last = performance.now();
      glide = { frame: 0 };
      var step = function (now) {
        var dt = now - last;
        last = now;
        velocity *= Math.pow(0.95, dt / 16);
        if (Math.abs(velocity) < 0.02 || !scrollBy(pan.el, pan.vertical, velocity * dt)) { glide = null; return; }
        glide.frame = requestAnimationFrame(step);
      };
      glide.frame = requestAnimationFrame(step);
    };

    // The mouse's primary button, or a pen, is a finger.
    addEventListener('pointerdown', function (e) {
      if (e.pointerType === 'touch' || e.button !== 0 || drag) return;
      stopGlide();
      var target = targetOf(e);
      var t = finger(++ids, target, e.clientX, e.clientY, e.screenX, e.screenY);
      drag = { pointer: e.pointerId, target: target, id: t.identifier, x: e.clientX, y: e.clientY, lastX: e.clientX, lastY: e.clientY, moved: false, samples: [] };
      swallow = false;
      drag.prevented = fire('touchstart', target, t, e);
    }, true);
    addEventListener('pointermove', function (e) {
      if (!drag || e.pointerId !== drag.pointer) return;
      var dx = e.clientX - drag.lastX, dy = e.clientY - drag.lastY;
      if (!drag.moved) {
        if (Math.abs(e.clientX - drag.x) < slop && Math.abs(e.clientY - drag.y) < slop) return;
        drag.moved = true;
        var vertical = Math.abs(dy) >= Math.abs(dx);
        if (!drag.prevented && pannable(drag.target, vertical)) {
          var el = scrollerOf(drag.target, vertical, -(vertical ? dy : dx));
          if (el) {
            drag.pan = { el: el, vertical: vertical };
            unsnap(drag.pan);
          }
        }
      }
      drag.lastX = e.clientX;
      drag.lastY = e.clientY;
      if (fire('touchmove', drag.target, finger(drag.id, drag.target, e.clientX, e.clientY, e.screenX, e.screenY), e)) drag.claimed = true;
      var pan = drag.pan;
      if (pan && !drag.claimed) {
        var by = -(pan.vertical ? dy : dx);
        scrollBy(pan.el, pan.vertical, by);
        drag.samples.push({ t: e.timeStamp, by: by });
        while (e.timeStamp - drag.samples[0].t > 100) drag.samples.shift();
      }
    }, true);
    var end = function (e, kind) {
      var d = drag;
      drag = null;
      var x = e ? e.clientX : d.lastX, y = e ? e.clientY : d.lastY;
      var prevented = fire(kind, d.target, finger(d.id, d.target, x, y, e ? e.screenX : 0, e ? e.screenY : 0), e);
      swallow = d.moved || d.prevented || prevented;
      if (d.pan && !d.claimed) {
        var sum = 0, first = d.samples.length ? d.samples[0].t : 0, last = first;
        d.samples.forEach(function (s) { sum += s.by; last = s.t; });
        fling(d.pan, last > first && (e ? e.timeStamp : last) - last < 50 ? sum / (last - first) : 0);
      }
    };
    addEventListener('pointerup', function (e) { if (drag && e.pointerId === drag.pointer) end(e, 'touchend'); }, true);
    addEventListener('pointercancel', function (e) { if (drag && e.pointerId === drag.pointer) end(e, 'touchcancel'); }, true);
    addEventListener('blur', function () { if (drag) end(null, 'touchcancel'); });

    // A phone sends no mouse events while a finger moves, none for a touch the page prevented, and no click after a drag. A mouseup follows a mousedown the page saw, so its own mouse handling does not stay pressed. A drag selects no text and drags no image, as hiding its moves keeps the engine from extending a selection, while a press still places the caret, which the engine does through selectstart.
    var hide = function (e) {
      e.stopImmediatePropagation();
      e.preventDefault();
    };
    var hideUp = false;
    addEventListener('mousedown', function (e) { hideUp = !!(drag && drag.prevented); if (hideUp) hide(e); }, true);
    addEventListener('mousemove', function (e) { if (drag) hide(e); }, true);
    addEventListener('mouseup', function (e) { if (hideUp) { hideUp = false; hide(e); } }, true);
    addEventListener('click', function (e) { if (swallow) { swallow = false; hide(e); } }, true);
    addEventListener('selectstart', function (e) { if (drag && drag.moved) e.preventDefault(); }, true);
    addEventListener('dragstart', function (e) { if (drag) e.preventDefault(); }, true);
  }

  // A wheel or trackpad scroll that nothing under it would scroll and the page leaves alone is a finger's swipe: it lifts once the wheel rests or has moved the finger a quarter of the view, and what the scroll has left, such as its momentum, goes nowhere.
  var lift = function (w) {
    clearTimeout(w.timer);
    if (!w.lifted) {
      w.lifted = true;
      fire('touchend', w.target, finger(w.id, w.target, w.x, w.y, 0, 0), null);
    }
  };
  addEventListener('wheel', function (e) {
    if (e.ctrlKey || drag || e.defaultPrevented) return;
    var unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? innerHeight : 1;
    var dx = e.deltaX * unit, dy = e.deltaY * unit;
    var vertical = Math.abs(dy) >= Math.abs(dx);
    if (!wheel && scrollerOf(targetOf(e), vertical, vertical ? dy : dx)) return;
    e.preventDefault();
    if (!wheel) {
      stopGlide();
      var target = targetOf(e);
      wheel = { target: target, id: ++ids, x: e.clientX, y: e.clientY, fromX: e.clientX, fromY: e.clientY };
      fire('touchstart', target, finger(wheel.id, target, wheel.x, wheel.y, e.screenX, e.screenY), e);
    }
    var w = wheel;
    clearTimeout(w.timer);
    w.timer = setTimeout(function () {
      lift(w);
      wheel = null;
    }, w.lifted ? 250 : 150);
    if (w.lifted) return;
    w.x = Math.min(Math.max(w.x - dx, 0), innerWidth - 1);
    w.y = Math.min(Math.max(w.y - dy, 0), innerHeight - 1);
    fire('touchmove', w.target, finger(w.id, w.target, w.x, w.y, 0, 0), e);
    if (Math.abs(w.y - w.fromY) >= innerHeight / 4 || Math.abs(w.x - w.fromX) >= innerWidth / 4) {
      lift(w);
      w.timer = setTimeout(function () { wheel = null; }, 250);
    }
  }, { passive: false });
})
