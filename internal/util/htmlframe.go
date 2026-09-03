package util

import (
	"strings"

	"golang.org/x/net/html"
)

const htmlFrameViewport = `<meta name="viewport" content="width=device-width, initial-scale=1">`

// HTMLFrameDocumentVersion invalidates prepared-document caches whenever the
// bridge protocol changes.
const HTMLFrameDocumentVersion = "3"

const htmlFrameBridge = `<script>(function(){
var protocolVersion=` + HTMLFrameDocumentVersion + `;
var sizeMessageType="myblog:html-size";
var readyMessageType="myblog:html-ready";
var measureMessageType="myblog:measure-html";
var viewportMessageType="myblog:html-viewport";
var readyImageRatio=.75;
var minimumReadyDelay=360;
var readyQuietWindow=320;
var maximumReadyWait=8000;
var startedAt=Date.now();
var lastHeight=0;
var measureScheduled=false;
var readyTimer=0;
var hardTimer=0;
var readySent=false;
var domReady=document.readyState!=="loading";
var pageLoaded=document.readyState==="complete";
var fontsReady=!document.fonts||!document.fonts.ready;
function pageSize(){
  var root=document.documentElement;
  var body=document.body;
  return {
    height:Math.ceil(Math.max(
      root?root.scrollHeight:0,root?root.offsetHeight:0,
      body?body.scrollHeight:0,body?body.offsetHeight:0
    )),
    width:Math.ceil(Math.max(
      root?root.scrollWidth:0,root?root.offsetWidth:0,
      body?body.scrollWidth:0,body?body.offsetWidth:0
    ))
  };
}
function postSize(force){
  var size=pageSize();
  if(!Number.isFinite(size.height)||size.height<1)return size;
  if(!force&&Math.abs(size.height-lastHeight)<2)return size;
  lastHeight=size.height;
  parent.postMessage({type:sizeMessageType,version:protocolVersion,height:size.height,width:size.width},"*");
  return size;
}
function measure(){
  measureScheduled=false;
  postSize(false);
}
function scheduleMeasure(){
  if(measureScheduled)return;
  measureScheduled=true;
  requestAnimationFrame(measure);
}
function imageProgress(){
  var images=document.images;
  if(!images||!images.length)return 1;
  var viewportHeight=Math.max(document.documentElement?document.documentElement.clientHeight:0,window.innerHeight||0);
  var relevant=0;
  var completed=0;
  for(var index=0;index<images.length;index++){
    var image=images[index];
    var rect=image.getBoundingClientRect();
    var nearViewport=rect.bottom>=-viewportHeight*.5&&rect.top<=viewportHeight*1.5;
    if(image.loading==="lazy"&&!nearViewport)continue;
    relevant++;
    if(image.complete)completed++;
  }
  return relevant?completed/relevant:1;
}
function revealViewportTargets(payload){
  var top=Number(payload.top);
  var height=Number(payload.height);
  if(!Number.isFinite(top)||!Number.isFinite(height)||height<1)return;
  var margin=Math.max(48,height*.08);
  var bottom=top+height;
  var targets=document.querySelectorAll("[data-reveal]");
  for(var index=0;index<targets.length;index++){
    var rect=targets[index].getBoundingClientRect();
    if(rect.bottom>=top-margin&&rect.top<=bottom+margin){
      targets[index].classList.add("is-visible");
    }
  }
}
function reportReady(reason){
  var size=postSize(true);
  parent.postMessage({
    type:readyMessageType,
    version:protocolVersion,
    height:size.height,
    width:size.width,
    imageRatio:imageProgress(),
    reason:reason
  },"*");
}
function reveal(reason){
  if(readySent)return;
  readySent=true;
  clearTimeout(readyTimer);
  clearTimeout(hardTimer);
  requestAnimationFrame(function(){
    requestAnimationFrame(function(){reportReady(reason);});
  });
}
function canReveal(){
  return domReady&&pageLoaded&&fontsReady&&imageProgress()>=readyImageRatio;
}
function scheduleReady(){
  if(readySent)return;
  clearTimeout(readyTimer);
  if(!canReveal())return;
  var remaining=Math.max(0,minimumReadyDelay-(Date.now()-startedAt));
  readyTimer=setTimeout(function(){
    if(canReveal())reveal("stable");
  },Math.max(readyQuietWindow,remaining));
}
function documentChanged(){
  scheduleMeasure();
  scheduleReady();
}
addEventListener("message",function(event){
  if(event.source!==parent||!event.data||event.data.version!==protocolVersion)return;
  if(event.data.type===viewportMessageType){
    revealViewportTargets(event.data);
    return;
  }
  if(event.data.type===measureMessageType){
    lastHeight=0;
    scheduleMeasure();
    if(readySent)reportReady("resume");
  }
});
if(!domReady)addEventListener("DOMContentLoaded",function(){domReady=true;documentChanged();},{once:true});
if(!pageLoaded)addEventListener("load",function(){pageLoaded=true;documentChanged();},{once:true});
if(window.ResizeObserver){
  var observer=new ResizeObserver(documentChanged);
  observer.observe(document.documentElement);
  if(document.body)observer.observe(document.body);
}
if(window.MutationObserver){
  new MutationObserver(documentChanged).observe(document.documentElement,{
    subtree:true,childList:true,attributes:true,characterData:true
  });
}
if(document.fonts&&document.fonts.ready){
  document.fonts.ready.then(function(){fontsReady=true;documentChanged();},function(){fontsReady=true;documentChanged();});
}
hardTimer=setTimeout(function(){reveal("timeout");},maximumReadyWait);
setTimeout(documentChanged,0);
setTimeout(documentChanged,300);
setTimeout(documentChanged,1200);
})();</script>`

// PrepareHTMLForFrame preserves the uploaded document while adding only the
// viewport and size-reporting bridge required by the sandboxed host page.
func PrepareHTMLForFrame(source string) string {
	hasViewport, hasHead := inspectHTMLFrameDocument(source)
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	var output strings.Builder
	output.Grow(len(source) + len(htmlFrameBridge) + len(htmlFrameViewport))
	viewportAdded := hasViewport
	bridgeAdded := false

	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if tokenType == html.EndTagToken && token.Data == "head" && !viewportAdded {
			output.WriteString(htmlFrameViewport)
			viewportAdded = true
		}
		if tokenType == html.StartTagToken && token.Data == "body" && !viewportAdded && !hasHead {
			output.WriteString(htmlFrameViewport)
			viewportAdded = true
		}
		if tokenType == html.EndTagToken && token.Data == "body" && !bridgeAdded {
			output.WriteString(htmlFrameBridge)
			bridgeAdded = true
		}
		output.Write(tokenizer.Raw())
	}
	if !viewportAdded {
		output.WriteString(htmlFrameViewport)
	}
	if !bridgeAdded {
		output.WriteString(htmlFrameBridge)
	}
	return output.String()
}

func inspectHTMLFrameDocument(source string) (hasViewport, hasHead bool) {
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "head":
				hasHead = true
			case "meta":
				for _, attribute := range token.Attr {
					if strings.EqualFold(attribute.Key, "name") && strings.EqualFold(attribute.Val, "viewport") {
						hasViewport = true
					}
				}
			}
		}
	}
}
