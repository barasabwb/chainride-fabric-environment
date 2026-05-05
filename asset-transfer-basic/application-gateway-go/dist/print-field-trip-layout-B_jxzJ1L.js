import{H as s,f as c,T as S,c as v,h as A,k,l as L,i as $,r as H,C as M,m as T,j as e,B as O,I as B,P as E,n as _}from"./index-cOPCOqUf.js";import{a as U,c as G,P as K}from"./print-D2XaCMCy.js";import"./en-US-ivqE8J9H.js";const V=s.div`
  font-size: 16px;
  line-height: 115%;
  margin: 8px;
`,W=s.div``,J=s.h1`
  border-bottom: 3px solid gray;
  font-size: 30px;
  font-weight: bold;
`,Y=s.ul`
  font-size: 16px;
  list-style: none;
  margin-top: 1em;
  padding: 0;
`,r=s.span`
  :empty:before {
    content: 'N/A';
  }
`,Q=s.div`
  background: ${c[100]};
  margin-top: 1em;

  & > h2 {
    font-size: 20px;
    font-weight: bold;
    margin: 0;
    padding: 4px;
  }
`,b=s.div`
  padding: 8px;
`,f=c[800],X=s.div`
  border: 3px solid ${f};
  margin-top: 0.5em;

  & > h3 {
    background: ${f};
    color: white;
    font-size: 18px;
    font-weight: bold;
    margin: 0;
    padding: 4px;
  }
`,Z=s.div`
  background: white;
  padding: 12px;
`,ee=s(S)`
  background: ${c[50]};
  border: 1px solid ${c[200]};
  border-radius: 0;
  margin-top: 15px;
  padding: 5px;
`;class ie extends H.Component{static contextType=M;_print=()=>{window.print()};componentDidMount(){const{initializeModules:t,intl:i}=this.props;U(),t(i)}componentDidUpdate(t){const{fetchFieldTripDetails:i,intl:d,receivedFieldTrips:l,request:n,requestId:a,session:p}=this.props;if(!t.session&&p&&(l({fieldTrips:[{endTime:0,id:a}]}),i(a,d)),n&&n!==t.request){const{endLocation:h,schoolName:x}=n;document.title=`Field Trip: ${x} to ${h}`}}componentWillUnmount(){G()}render(){const{config:t,request:i}=this.props,{LegIcon:d}=this.context;if(!i)return null;const{address:l,classpassId:n,emailAddress:a,endLocation:p,faxNumber:h,grade:x,numChaperones:y,numFreeStudents:I,numStudents:P,phoneNumber:C,schoolName:u,teacherName:D,timeStamp:F}=i,q=[{title:"Outbound Trip (to Destination)",trip:T(i,!0),tripAbsentMessage:"No Outbound Trip Planned"},{title:"Inbound Trip (from Destination)",trip:T(i,!1),tripAbsentMessage:"No Inbound Trip Planned"}];return e.jsxs(V,{children:[e.jsxs(W,{children:[e.jsx(O,{bsSize:"small",onClick:this._print,style:{float:"right"},children:e.jsx(B,{Icon:E,children:"Print"})}),e.jsxs(J,{children:["Field Trip Plan: ",u," to ",p]})]}),e.jsxs(Y,{children:[e.jsxs("li",{children:[e.jsx("b",{children:"Teacher"}),": ",e.jsx(r,{children:D})," (",u,", Grade:"," ",e.jsx(r,{children:x}),")"]}),e.jsxs("li",{children:[e.jsx("b",{children:"Teacher Address"}),": ",e.jsx(r,{children:l})]}),e.jsxs("li",{children:[e.jsx("b",{children:"Phone"}),": ",e.jsx(r,{children:C})," / ",e.jsx("b",{children:"Fax"}),":"," ",e.jsx(r,{children:h})]}),e.jsxs("li",{children:[e.jsx("b",{children:"Email"}),": ",e.jsx(r,{children:a})]}),e.jsxs("li",{children:[e.jsx("b",{children:"Students Age 7 and Over"}),": ",P||0]}),e.jsxs("li",{children:[e.jsx("b",{children:"Students Age 6 and Under"}),": ",I||0]}),e.jsxs("li",{children:[e.jsx("b",{children:"Chaperones"}),": ",y||0]}),n&&e.jsxs("li",{children:[e.jsx("b",{children:"Class Pass Hop Card #"}),": ",n]}),e.jsx("li",{children:e.jsxs("i",{children:["Request submitted: ",F]})})]}),q.map(({title:w,trip:m,tripAbsentMessage:z},N)=>e.jsxs(Q,{children:[e.jsx("h2",{children:w}),m?m.groupItineraries?.map((j,R)=>{const g=JSON.parse(_(j.itinData));return e.jsx(b,{children:e.jsxs(X,{children:[e.jsxs("h3",{children:[j.passengers," passengers on following itinerary:"]}),e.jsxs(Z,{children:[e.jsx(K,{config:t,itinerary:g,LegIcon:d}),e.jsx(ee,{itinerary:g})]})]})},R)}):e.jsx(b,{children:e.jsx("i",{children:z})})]},N))]})}}const se=(o,t)=>{const i=parseInt(o.router.location.query.requestId),{requests:d}=o.callTaker.fieldTrip,l=d.data.find(n=>n.id===i);return{config:o.otp.config,request:l,requestId:i,session:o.callTaker.session}},ne={fetchFieldTripDetails:L,initializeModules:k,receivedFieldTrips:A},de=v(se,ne)($(ie));export{de as default};
