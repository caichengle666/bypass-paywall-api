(() => {
  // 1. Unhide paywall
  document.querySelectorAll(".paywall,[class*='Paywalled'],[class*='paywall'],[class*='subscription-prompt']").forEach(function(el){
    el.hidden=false; el.removeAttribute("hidden"); el.removeAttribute("aria-hidden");
    el.style.cssText="display:block!important;visibility:visible!important;opacity:1!important;max-height:none!important;height:auto!important;overflow:visible!important";
  });
  document.querySelectorAll("html,body").forEach(function(el){if(el&&el.style){el.style.overflow="visible";el.style.maxHeight="none";}});

  // 2. Article paragraphs
  var paras=[], seen={};
  var containers = document.querySelectorAll("article,.article-body__content,.article__body,.paywall,[class*='Paywalled'],[class*='article-body'],[role='article'],main,.story-body,.story-content");
  if(!containers.length) containers=[document.body||document.documentElement];
  containers.forEach(function(c){
    if(!c)return;
    c.querySelectorAll("p,div.p,div.paragraph").forEach(function(p){
      var t=(p.innerText||"").trim();
      if(!t||t.length<10||seen[t])return;
      seen[t]=true;
      var l=t.toLowerCase();
      if(/copyright|subscribe|sign up|订阅|登录|your browser does not support|already a subscriber/.test(l))return;
      paras.push(t);
    });
  });
  if(paras.length<3){
    document.querySelectorAll("p").forEach(function(p){
      var t=(p.innerText||"").trim();
      if(!t||t.length<10||seen[t])return;
      seen[t]=true;
      if(!/copyright|subscribe/.test(t.toLowerCase()))paras.push(t);
    });
  }

  // 3. Navigation
  var navMap={};
  document.querySelectorAll("nav a,nav button,[role='navigation'] a,header a,[class*='nav' i] a").forEach(function(a){
    var t=(a.innerText||"").trim();
    var h=a.getAttribute("href")||"";
    if(t.length>1&&t.length<30&&t!==h&&!navMap[t]){navMap[t]={label:t,href:h};}
  });

  // 4. Section-organized links
  var urlMap={};
  document.querySelectorAll("a[href]").forEach(function(a){
    var href=a.getAttribute("href");
    if(!href)return;
    try{var u=new URL(href,window.location.href);href=u.href;}catch(e){return;}
    if(!href.startsWith("http"))return;
    if(href.indexOf("#")>=0)href=href.split("#")[0];
    var l=href.toLowerCase();
    if(/(search|login|subscribe|signup|register|password|privacy|terms|cookie|careers|about|contact)/.test(l))return;
    if(/doubleclick\.net|googlesyndication|googleadservices|amazon-adsystem|outbrain\.com/.test(l))return;
    var t=(a.innerText||"").trim();
    if(t.length<5)return;

    // Section from DOM
    var sec="";
    var p=a.closest("[data-sub_type]");
    if(p)sec=p.getAttribute("data-sub_type");
    if(!sec){
      var p2=a.closest("section,div[class*='section'],li[class*='item'],article");
      var d=5;
      while(p2&&p2!==document.body&&d-->0){
        var h=p2.querySelector("h2,h3,h4,[class*='headline' i],[class*='section-title' i]");
        if(h){sec=(h.innerText||"").trim().substring(0,50);break;}
        p2=p2.parentElement;
      }
    }

    // Parent heading
    var p3=a.closest("h1,h2,h3,h4,h5");
    if(p3){var pt=(p3.innerText||"").trim();if(pt.length>t.length)t=pt;}

    if(!urlMap[href]||t.length>urlMap[href].title.length) urlMap[href]={url:href,title:t,section:sec||"General"};
  });

  var all=Object.values(urlMap);
  var groups={}, titleDedup={};
  all.forEach(function(l){
    if(!groups[l.section]) groups[l.section]=[];
    var k=l.title.substring(0,25);
    if(titleDedup[k+"|"+l.section])return;
    titleDedup[k+"|"+l.section]=true;
    groups[l.section].push({title:l.title,url:l.url});
  });

  var sections=[];
  var known=["economics","politics","world","china","tech","markets","opinion","business","cn-economy","cn-china","cn-technology","cn-markets","cn-opinion","international","finance"];
  Object.keys(groups).filter(function(k){return k!=="General"&&groups[k].length>0;}).forEach(function(k){
    sections.push({section:k,articles:groups[k].slice(0,15)});
  });
  if(groups["General"]&&groups["General"].length>0){
    sections.push({section:"General",articles:groups["General"].slice(0,20)});
  }
  sections.sort(function(a,b){
    var ai=known.indexOf(a.section.toLowerCase());if(ai<0)ai=999;
    var bi=known.indexOf(b.section.toLowerCase());if(bi<0)bi=999;
    return ai-bi;
  });

  var totalArticles=sections.reduce(function(s,sec){return s+sec.articles.length;},0);
  var homepageData = totalArticles>10 ? {navigation:Object.values(navMap).slice(0,25),sections:sections,totalArticles:totalArticles} : null;

  return {
    title:(document.querySelector("h1")||{}).innerText||document.title,
    site:window.location.hostname,
    pageUrl:window.location.href,
    paragraphs:paras,
    paragraphCount:paras.length,
    sections:homepageData
  };
})()
