import{c as p,u as c,r as x,C as L,j as e,o as y,E as j,G as w,q as C,t as R,N as b,w as f,v as E,H as P,R as M}from"./index-cOPCOqUf.js";import{T as N}from"./trip-preview-layout-base-B2yoLzd_.js";import"./Map.esm-BGS8DvGN.js";import"./print-D2XaCMCy.js";import"./en-US-ivqE8J9H.js";const S=({config:t,itinerary:o})=>{const a=c(),{getTransitiveRouteLabel:s}=x.useContext(L),{baseLayers:i,initLat:m=0,initLon:l=0,initZoom:u,maxZoom:d,navigationControlPosition:h="bottom-right",transitive:v}=t.map||{},n=i?.map(g=>g.url),{disableFlexArc:T}=v||{},{legs:r=[]}=o||{};return e.jsxs(y,{baseLayer:(n?.length||0)>1?n:n?.[0],center:[m,l],mapLibreProps:{reuseMaps:!0},maxZoom:d,zoom:u,children:[e.jsx(j,{fromLocation:r[0]?.from,toLocation:r[r.length-1]?.to}),e.jsx(w,{position:"top-left"}),o&&e.jsx(C,{transitiveData:R(o,{companies:t.companies,disableFlexArc:T,getRouteLabel:s,intl:a})}),e.jsx(b,{position:h})]})},U=t=>({config:t.otp.config}),I=p(U)(S),O=P.div`
  height: 100%;
  width: 100%;

  .map {
    height: 100%;
    width: 100%;
  }
`,_=({monitoredTrip:t})=>{const a=c().formatMessage({id:"components.TripPreviewLayout.previewTrip"}),s=t?.journeyState?.matchingItinerary||t?.itinerary;return e.jsx(N,{header:a,itinerary:s,mapElement:e.jsx(O,{className:"map-container percy-hide",children:e.jsx(I,{itinerary:s})}),subTitle:t?.tripName,title:a})},q=(t,o)=>{const{loggedInUserMonitoredTrips:a}=t.user,s=o.match.params.id;return{monitoredTrip:a?.find(i=>i.id===s)}},$=f(E(p(q)(_),M),!0);export{$ as default};
