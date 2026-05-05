import{j as e,aU as x,M as l,H as u,ca as f,cb as g,cc as b,f as S,u as p}from"./index-ODYXt4nQ.js";import{b as v,c as y}from"./form-navigation-buttons-B1leNGVd.js";const m=u(f)`
  padding: 0.2em 0.5em 0.2em !important;
`;function F(s){switch(s?.toLowerCase()){case"pending":return e.jsx(m,{bsStyle:"warning",children:e.jsx(l,{id:"components.StatusBadge.pending"})});case"confirmed":case"verified":return e.jsx(m,{style:{background:"green"},children:e.jsx(l,{id:"components.StatusBadge.verified"})});case"invalid":return e.jsx(m,{children:e.jsx(l,{id:"components.StatusBadge.invalid"})});default:return null}}const O=({status:s})=>e.jsxs(e.Fragment,{children:[e.jsx(x,{children:" ("}),F(s),e.jsx(x,{children:") "})]}),M=u.summary`
  /* Revert display:block set by Bootstrap that hides the native expand/collapse caret. */
  display: revert-layer;
  font-size: 24px;
  /* Format summary as labels */
  font-weight: 500;
  margin-bottom: 0.5em;
  margin-top: 0.5em;

  &::before {
    content: '';
    display: inline-block;
    width: 0.5em; /* Adjust this value to increase or decrease space */
  }
`,B=u.div`
  color: ${S[700]};
`,C=({canceling:s,panes:r,subtitle:i,title:a})=>e.jsxs(e.Fragment,{children:[e.jsx(g,{children:a}),i&&e.jsx(B,{children:i}),r.map(({collapsible:t,hidden:o,pane:n,props:c,title:d},h)=>!o&&e.jsx(b,{children:t?e.jsxs("details",{children:[e.jsx(M,{children:d}),e.jsx(n,{canceled:s,...c})]}):e.jsxs(e.Fragment,{children:[d&&e.jsx("h3",{children:d}),e.jsx("div",{children:e.jsx(n,{canceled:s,...c})})]})},h))]}),H=({Control:s=y,children:r,defaultValue:i,label:a,name:t,onChange:o})=>{const n={as:s,componentClass:"select",defaultValue:i,id:t,name:t,onChange:o};return o||delete n.onChange,e.jsxs(e.Fragment,{children:[a&&e.jsx("label",{htmlFor:t,children:a}),e.jsx(v,{...n,children:r})]})};function j({defaultValue:s,hideDefaultIndication:r,options:i}){const a=p();return e.jsx(e.Fragment,{children:i.map(({text:t,value:o},n)=>e.jsx("option",{value:o,children:!r&&o===s?a.formatMessage({id:"common.forms.defaultValue"},{value:t}):t},n))})}const k=[{id:"yes",value:"true"},{id:"no",value:"false"}];function P({defaultValue:s,hideDefaultIndication:r}){const i=p(),a=k.map(({id:t,value:o})=>({text:i.formatMessage({id:`common.forms.${t}`}),value:o}));return e.jsx(j,{defaultValue:(s||!1).toString(),hideDefaultIndication:r,options:a})}function z({decoratorFunc:s,defaultValue:r,minuteOptions:i}){const a=p(),t=i.map(n=>({text:n===60?a.formatMessage({id:"components.TripNotificationsPane.oneHour"}):a.formatMessage({id:"common.time.tripDurationFormat"},{hours:0,minutes:n,seconds:0}),value:n})),o=s?t.map(({text:n,value:c})=>({text:s(n,a),value:c})):t;return e.jsx(j,{defaultValue:r,options:o})}export{z as D,j as O,O as S,P as Y,C as a,H as b};
