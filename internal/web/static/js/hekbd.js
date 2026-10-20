(function(g){"use strict";
var D=typeof document!=="undefined"?document:null,N=0,Q="[data-hk]";
function norm(s){return String(s).replace(/\p{Cf}/gu,"").normalize("NFC").replace(/[\s\u0085]+/g," ").replace(/^ | $/g,"")}
function edit(v,s,e,t){if(s==null)s=v.length;if(e==null)e=s;if(!t&&s===e&&s>0)s-=/[\uDC00-\uDFFF]/.test(v[s-1])&&s>1?2:1;return[v.slice(0,s)+t+v.slice(e),s+t.length]}
function pick(ev,m){var k=ev.key;return!ev.ctrlKey&&!ev.metaKey&&!ev.altKey&&!ev.isComposing&&k&&k.length===1&&k.charCodeAt(0)<128&&m[ev.code]||""}
function hints(n,c){return c==="1"||(c!=="0"&&n<=3)}
function jar(s){var m=/(?:^|; )hekbd=(\d+):([01]?)/.exec(s||"");return m?[+m[1],m[2]]:[0,""]}
function put(el,t){var v=el.value,r=edit(v,el.selectionStart,el.selectionEnd,t);el.value=r[0];if(el.setSelectionRange)el.setSelectionRange(r[1],r[1]);el.dispatchEvent(new Event("input",{bubbles:true}))}
g.HebrewKeyboard={norm:norm,edit:edit,pick:pick,hints:hints,jar:jar,put:put};
if(typeof module==="object"&&module.exports)module.exports=g.HebrewKeyboard;
if(!D||g.__hekbd)return;g.__hekbd=1;
function save(c){D.cookie="hekbd="+N+":"+c+"; path=/; max-age=34560000; samesite=lax"+(location.protocol==="https:"?"; secure":"")}
function show(r,on){r.setAttribute("data-hints",on?"on":"off");var b=r.querySelector("[data-hk-hints]");if(b)b.setAttribute("aria-pressed",on?"true":"false")}
function boxFor(t){var a=D.querySelectorAll(Q);for(var i=0;i<a.length;i++)if(a[i].getAttribute("data-target")===t.id)return a[i]}
function keys(r){var m={};r.querySelectorAll("[data-code]").forEach(function(b){m[b.getAttribute("data-code")]=b.getAttribute("data-ch")});return m}
var seen;try{seen=sessionStorage.getItem("hekbd")}catch(x){}
function init(){var a=jar(D.cookie);N=a[0];if(!seen){seen=1;N++;save(a[1]);try{sessionStorage.setItem("hekbd","1")}catch(x){}}
D.querySelectorAll(Q).forEach(function(r){if(r.hasAttribute("data-ready"))return;r.setAttribute("data-ready","");show(r,hints(N,a[1]));var t=D.getElementById(r.getAttribute("data-target"));if(!t)return;t.setAttribute("data-hk-input","");t.setAttribute("dir","rtl");t.setAttribute("lang","he");if(matchMedia("(pointer: coarse)").matches)t.setAttribute("inputmode","none")})}
D.addEventListener("mousedown",function(e){if(e.target.closest&&e.target.closest(Q+" button"))e.preventDefault()});
D.addEventListener("click",function(e){var b=e.target.closest&&e.target.closest(Q+" button");if(!b)return;var r=b.closest(Q);
if(b.hasAttribute("data-hk-hints")){var on=r.getAttribute("data-hints")!=="on";show(r,on);save(on?"1":"0");return}
var t=D.getElementById(r.getAttribute("data-target")),k=b.getAttribute("data-act");if(!t)return;
if(k==="enter"){if(t.dispatchEvent(new CustomEvent("hekbd:enter",{bubbles:true,cancelable:true}))&&t.form)t.form.requestSubmit();return}
put(t,k==="space"?" ":k==="back"?"":b.getAttribute("data-ch"));if(e.detail&&D.activeElement!==t)t.focus({preventScroll:true})});
D.addEventListener("keydown",function(e){var t=e.target,r=t&&t.id&&boxFor(t);if(!r||!e.code)return;var b=r.querySelector('[data-code="'+e.code+'"]');if(!b)return;
b.setAttribute("data-pressed","");setTimeout(function(){b.removeAttribute("data-pressed")},150);var c=pick(e,keys(r));if(c){e.preventDefault();put(t,c)}});
D.addEventListener("submit",function(e){if(e.target.querySelectorAll)e.target.querySelectorAll("[data-hk-input]").forEach(function(i){var n=norm(i.value);if(n!==i.value)i.value=n})},true);
D.addEventListener("htmx:configRequest",function(e){var p=e.detail.parameters;D.querySelectorAll("[data-hk-input][name]").forEach(function(i){var n=i.getAttribute("name");if(p[n]!=null)p[n]=norm(p[n])})});
D.addEventListener("htmx:afterSwap",init);
if(D.readyState==="loading")D.addEventListener("DOMContentLoaded",init);else init()
})(typeof window!=="undefined"?window:globalThis);
