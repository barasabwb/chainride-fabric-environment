import{H as p,bn as W,cd as z,ce as g,cf as Y,c as k,cg as K,i as M,r as b,ch as y,ci as X,C as J,cj as R,bc as Q,j as e,M as d,ck as Z,cl as E,cm as ee,cn as U,bA as B,aU as te,d as ae,co as $,w as oe,v as se,R as ne,cp as ie,bF as re,bO as ce,cq as D,aT as v,bL as le,bw as T,x as de,cb as me,aV as _,I as pe}from"./index-cOPCOqUf.js";import{C as I,b as w,c as P,H as N,a as G,S as j,O as he,d as ue,e as xe,F as ge,T as fe}from"./form-navigation-buttons-COzrSgxp.js";import{A as ve}from"./account-page-LIHruvCY.js";import{F as O,a as Pe}from"./formatted-validation-error-B1HGRYwL.js";const be=p(W).attrs({className:"btn-group"})`
  label::first-letter {
    text-transform: uppercase;
  }

  input {
    clip: rect(0, 0, 0, 0);
    height: 0;
    /* Firefox will still render (tiny) controls, even if their bounds are empty,
       so move them out of sight. */
    left: -20px;
    position: relative;
    width: 0;
  }

  input:focus + label {
    outline: 5px auto blue;
    /* This next line enhances the visuals in Chromium (webkit) browsers */
    outline: 5px auto -webkit-focus-ring-color;
    outline-offset: -2px;
  }

  /* Without !important, round borders get overwritten by bootstrap's CSS. */
  label:first-of-type {
    border-bottom-left-radius: 4px !important;
    border-top-left-radius: 4px !important;
  }

  /* This is to have the between-labels border to be 1px. */
  label:not(label:first-of-type) {
    margin-left: -1px;
  }
`,je=p(Y)`
  border-bottom-color: #ccc;
  display: block;
  margin-bottom: 0;
  width: 100%;

  ${g.ClearButton},
  ${g.DropdownButton} {
    display: none;
  }

  ${g.Input} {
    display: table-cell;
    font-size: 100%;
    line-height: 20px;
    padding: 0;
    width: 100%;

    ::placeholder {
      color: #676767;
    }
  }
  ${g.MenuItemList} {
    position: absolute;
    ${l=>l.static?"width: 100%;":""}
  }
  ${g.MenuItemLi} {
    overflow: hidden;
    ${l=>l.static?"padding-left: 10px; padding-right: 5px; width: 100%":""}

    &:focus,
    &:hover {
      color: inherit;
    }
  }
`,Ce=z(je,{actions:{getCurrentPosition:null},excludeSavedLocations:!0}),{isMobile:Se}=ae.ui,Fe=p(B)`
  margin-right: 5px;
`,Le=p.div`
  align-items: center;
  display: flex;
`,ye=p(be)`
  & > label {
    padding: 5px;
  }
`,A=p(G)`
  &.has-feedback label ~ .form-control-feedback {
    top: 28px;
  }
`;function Ee(l){const{address:t,lat:s,lon:a}=l;return{lat:s,lon:a,name:t}}const S=class S extends b.Component{constructor(){super(...arguments),this._setLocation=t=>{const{geocoderConfig:s,intl:a,setValues:o,values:i}=this.props,{category:n,lat:r,lon:c,name:u}=t,h=m=>{o({...i,address:m,lat:r,lon:c})};if(n==="CURRENT_LOCATION"){const m=a.formatMessage({id:"common.coordinates"},{lat:r,lon:c});h(m),y(s).reverse({point:{lat:r,lon:c}}).then(x=>{h(x.name)}).catch(x=>{console.warn("Reverse geocode failed:",x)})}else h(u)},this._handleLocationChange=t=>{this._setLocation(t.location)},this._handleGetCurrentPosition=()=>{const{geocoderConfig:t,getCurrentPosition:s,intl:a}=this.props;s(a,X,({coords:o})=>{const{latitude:i,longitude:n}=o;this._setLocation({category:"CURRENT_LOCATION",lat:i,lon:n}),y(t).reverse({point:o}).then(this._setLocation).catch(r=>{console.warn(r)})})}}render(){const{errors:t,geocoderConfig:s,intl:a,values:o}=this.props,{SvgIcon:i}=this.context,n=R(o),r=Q(this.props),c=a.formatMessage({id:"components.PlaceEditor.nameExample"}),u=o.name?.length||0,h=$-u,m=0-h,{geocoderResultsOrder:x}=s;return e.jsxs("div",{children:[!n&&e.jsxs(e.Fragment,{children:[e.jsxs(A,{validationState:r.name,children:[e.jsx(I,{htmlFor:"name",children:e.jsx(d,{id:"components.PlaceEditor.namePrompt"})}),e.jsx("div",{"aria-hidden":!0,style:{fontWeight:300,position:"absolute",right:0,top:0},children:m>0?e.jsx(d,{id:"components.FavoritePlaceScreen.charactersOverLimit",values:{chars:m}}):e.jsx(d,{id:"components.FavoritePlaceScreen.charactersRemaining",values:{chars:h}})}),e.jsx(w,{"aria-invalid":!!t.name,"aria-required":!0,as:P,id:"name",name:"name",placeholder:c}),e.jsx(P.Feedback,{}),e.jsxs(N,{role:"alert",children:[r.name&&e.jsx(O,{type:t.name}),e.jsx(Z.InvisibleAdditionalDetails,{children:m>0&&e.jsx(d,{id:"components.FavoritePlaceScreen.charactersOverLimit",values:{chars:m}})})]})]}),e.jsx(G,{children:e.jsxs(ye,{children:[e.jsx("legend",{children:e.jsx(d,{id:"components.PlaceEditor.locationTypePrompt"})}),Object.keys(E).map(F=>{const{icon:V,type:f}=E[F],H=ee(U(F,a)),L=`location-type-${f}`,q=o.type===f;return e.jsxs(b.Fragment,{children:[e.jsx(w,{id:L,name:"type",type:"radio",value:f}),e.jsxs("label",{className:q?"btn btn-primary active":"btn btn-default",htmlFor:L,children:[e.jsx(B,{size:"1.5x",children:e.jsx(i,{iconName:V})}),e.jsx(te,{children:H})]})]},f)})]})})]}),e.jsxs(A,{validationState:r.address,children:[e.jsx(I,{children:e.jsx(d,{id:"components.PlaceEditor.addressPrompt"})}),e.jsxs(Le,{children:[n&&e.jsx(Fe,{size:"1.5x",children:e.jsx(i,{iconName:o.icon})}),e.jsx(Ce,{className:"form-control",geocoderResultsOrder:x,getCurrentPosition:this._handleGetCurrentPosition,inputPlaceholder:n?a.formatMessage({id:"components.PlaceEditor.locationPlaceholder"},{placeName:o.name}):a.formatMessage({id:"components.PlaceEditor.genericLocationPlaceholder"}),isRequired:!0,location:Ee(o),locationType:"to",onLocationSelected:this._handleLocationChange,selfValidate:!!r.address,showClearButton:!1,static:Se()})]}),e.jsx(P.Feedback,{}),e.jsx(N,{role:"alert",children:r.address&&e.jsx(O,{type:t.address})})]})]})}};S.contextType=J;let C=S;const Te=l=>({geocoderConfig:l.otp.config.geocoder}),_e={getCurrentPosition:K},Ie=k(Te,_e)(M(C)),we={...D.custom,address:"",name:""},Ne=p.div`
  margin-bottom: 100px;
  @media (max-width: 768px) {
    margin-bottom: 50px;
  }
`,Oe={address:j().required("invalid-address"),name:j().required("invalid-name")};class Ae extends b.Component{state={pendingSave:!1};_handleSave=async t=>{this.setState({pendingSave:!0}),t.icon=D[t.type].icon;const{intl:s,placeIndex:a,saveUserPlace:o}=this.props;o(t,a,s),v()};_handleDelete=async t=>{const{deleteLoggedInUserPlace:s,intl:a,placeIndex:o}=this.props;window.confirm(a.formatMessage({id:"actions.user.confirmDeletePlace"}))&&(s(o,a,t),v())};_getPlaceToEdit=t=>{const{isNewPlace:s,placeIndex:a}=this.props;return s?we:t.savedLocations?t.savedLocations[a]:null};_getFullValidationSchema=(t,s)=>{const a=(s?t.filter(i=>!le(i.name)&&i.name!==s.name):t).map(i=>i.name),o=T(Oe);return o.name=j().required("invalid-name").notOneOf(a,"placename-already-used").max($,"placename-too-long"),he(o)};render(){const{intl:t,isCreating:s,isNewPlace:a,loggedInUser:o}=this.props,{pendingSave:i}=this.state,n=T(this._getPlaceToEdit(o));n?.name===null&&(n.name="");const r=n&&R(n);r&&(n.name=n.type);let c;return n?a?c=t.formatMessage({id:"components.FavoritePlaceScreen.addNewPlace"}):r?c=t.formatMessage({id:"components.FavoritePlaceScreen.editPlace"},{placeName:U(n.type,t)}):c=t.formatMessage({id:"components.FavoritePlaceScreen.editPlaceGeneric"}):c=t.formatMessage({id:"components.FavoritePlaceScreen.placeNotFound"}),e.jsxs(ve,{subnav:!s,children:[e.jsx(de,{title:[c,s?"":t.formatMessage({id:"components.SubNav.myAccount"})]}),e.jsx(ue,{initialValues:n,onSubmit:this._handleSave,validateOnBlur:!0,validateOnChange:!1,validationSchema:this._getFullValidationSchema(o.savedLocations,n),children:u=>e.jsxs(xe,{noValidate:!0,children:[e.jsx("div",{children:e.jsx(me,{as:s?"h1":"h2",children:c})}),e.jsx(Ne,{children:n?e.jsx(Ie,{...u}):e.jsx("p",{children:e.jsx(d,{id:"components.FavoritePlaceScreen.placeNotFoundDescription"})})}),e.jsx(ge,{backButton:{onClick:v,text:e.jsx(d,{id:"common.forms.back"})},extraButton:!a&&n.address&&{content:e.jsx(Pe,{onClick:()=>this._handleDelete(n),children:i?e.jsx(_,{}):e.jsx(pe,{Icon:fe,children:e.jsx(d,{id:"components.Place.deleteThisPlace"})})})},okayButton:n&&{text:i?e.jsx(_,{}):e.jsx(d,{id:"common.forms.save"}),type:"submit"}})]})})]})}}const ke=(l,t)=>{const{params:s,path:a}=t.match,o=s.id;return{isCreating:a.startsWith(ce),isNewPlace:o==="new",loggedInUser:l.user.loggedInUser,placeIndex:o}},Me={deleteLoggedInUserPlace:re,saveUserPlace:ie},De=oe(se(k(ke,Me)(M(Ae)),ne),!0);export{De as default};
