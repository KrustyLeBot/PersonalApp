// Thrown by the local services; the fetch router turns it into an HTTP-style response.
export class HttpError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

// Thrown when the app cannot read or write the data folder yet.
export class StorageAccessError extends Error {}
