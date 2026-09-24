export class APIError extends Error {constructor(public status:number,public code:string,message:string,public requestId=''){super(message);this.name='APIError'}}
