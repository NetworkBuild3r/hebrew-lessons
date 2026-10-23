(function(){var k="hebrew-theme",d=document,r=d.documentElement,on=d.addEventListener.bind(d);
function cur(){var t=r.getAttribute("data-theme");return t==="light"||t==="dark"?t:"auto"}
function paint(){var c=cur();d.querySelectorAll("[data-theme-choice]").forEach(function(b){b.setAttribute("aria-pressed",b.getAttribute("data-theme-choice")===c)})}
function apply(v){var a=v==="auto",go=function(){a?r.removeAttribute("data-theme"):r.setAttribute("data-theme",v);try{a?localStorage.removeItem(k):localStorage.setItem(k,v)}catch(e){}paint()};d.startViewTransition?d.startViewTransition(go):go()}
on("click",function(ev){var n=ev.target.closest?ev.target:ev.target.parentElement;if(!n)return;
var b=n.closest("[data-theme-choice]");if(b){apply(b.getAttribute("data-theme-choice"));return}
var s=n.closest("[data-sheet]");if(s){var m=d.getElementById(s.getAttribute("data-sheet"));if(m&&m.showModal){ev.preventDefault();m.showModal()}return}
if(n.closest("[data-close-sheet]")||n.tagName==="DIALOG")(n.closest("dialog")||n).close()});
on("keydown",function(ev){if(ev.ctrlKey||ev.metaKey||ev.altKey||/^(INPUT|TEXTAREA|SELECT)$/.test(ev.target.tagName))return;var i="1234".indexOf(ev.key),o=d.querySelectorAll(".lesson-frame .c-answer");if(i>=0&&o[i]){ev.preventDefault();o[i].click()}});
on("submit",function(ev){var f=ev.target;if(!f.hasAttribute("data-logout"))return;ev.preventDefault();fetch(f.action,{method:"POST",body:new URLSearchParams(new FormData(f)),redirect:"manual"}).finally(function(){location.assign(f.getAttribute("data-logout"))})});
on("DOMContentLoaded",paint);on("htmx:afterSwap",function(e){paint();var m=d.getElementById("how-sheet");if(e.detail.target.id=="how-body"&&m&&!m.open)m.showModal()});
if(!window.htmx)return;var c=htmx.config;c.globalViewTransitions=true;c.defaultSwapDelay=0;c.allowEval=false;c.includeIndicatorStyles=false;var dur=parseFloat(getComputedStyle(r).getPropertyValue("--dur-base"));if(typeof document.startViewTransition==="function")c.defaultSettleDelay=0;else if(dur)c.defaultSettleDelay=dur})();
