(() => {
  document.querySelectorAll(".paywall,[class*='Paywalled'],[class*='paywall']").forEach(function(el){
    el.hidden=false; el.removeAttribute("hidden"); el.removeAttribute("aria-hidden");
    el.style.cssText="display:block!important;visibility:visible!important;opacity:1!important;max-height:none!important;height:auto!important;overflow:visible!important";
  });
  document.querySelectorAll("html,body").forEach(function(el){el.style.overflow="visible";el.style.maxHeight="none";});
  var paras=[], seen={};
  var c=document.querySelectorAll(".paywall,[class*='Paywalled'],article,[role='article']");
  if(!c.length) c=[document];
  c.forEach(function(container){
    container.querySelectorAll("p").forEach(function(p){
      var t=(p.innerText||p.textContent||"").trim();
      if(t&&t.length>15&&!seen[t]){seen[t]=true;
        var l=t.toLowerCase();
        if(l.indexOf("subscribe")===-1&&l.indexOf("sign")===-1&&l.indexOf("copyright")===-1&&l.indexOf("all rights")===-1)
          paras.push(t);}
    });
  });
  return {title:(document.querySelector("h1")||{}).innerText||document.title,paragraphs:paras,paragraphCount:paras.length,pageUrl:window.location.href};
})()