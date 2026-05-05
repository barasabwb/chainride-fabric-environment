import{c as l,b5 as p,bf as a,H as m,B as u,r as v,u as x,j as e,aV as T,I as f,M as r,co as j,ae as S}from"./index-ODYXt4nQ.js";import{T as h}from"./form-navigation-buttons-B1leNGVd.js";const g=m(u)`
  background-color: white;
  border-color: ${a};
  color: ${a};
  :active,
  :focus,
  :focus:active,
  :hover {
    border-color: ${a};
    color: ${a};
  }
  float: right;
  margin-right: 10px;
`,y=({confirmAndDeleteUserMonitoredTrip:s,tripId:i})=>{const[o,t]=v.useState(!1),c=x(),d=async()=>{t(!0),await s(i,c),t(!1)};return e.jsx(g,{disabled:o,onClick:d,children:o?e.jsx(T,{}):e.jsx(f,{Icon:h,children:e.jsx(r,{id:"components.SavedTripEditor.deleteSavedTrip"})})})},D={confirmAndDeleteUserMonitoredTrip:p},E=l(null,D)(y);function n({type:s}){switch(s){case"invalid-address":return e.jsx(r,{id:"components.FavoritePlaceScreen.invalidAddress"});case"invalid-name":return e.jsx(r,{id:"components.FavoritePlaceScreen.invalidName"});case"placename-too-long":return e.jsx(r,{id:"components.FavoritePlaceScreen.nameTooLong",values:{char:j}});case"placename-already-used":return e.jsx(r,{id:"components.FavoritePlaceScreen.nameAlreadyUsed"});case"trip-name-already-used":return e.jsx(r,{id:"components.SavedTripScreen.tripNameAlreadyUsed"});case"trip-name-required":return e.jsx(r,{id:"components.SavedTripScreen.tripNameRequired"});case"select-at-least-one-day":return e.jsx(r,{id:"components.SavedTripScreen.selectAtLeastOneDay"});default:return null}}n.propTypes={type:S.string.isRequired};n.defaultProps={type:""};export{E as D,n as F,g as a};
