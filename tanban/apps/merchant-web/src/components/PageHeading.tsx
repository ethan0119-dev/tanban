import type { ReactNode } from 'react';

export function PageHeading({ title, description, extra }: { title: string; description?: string; extra?: ReactNode }) {
  return (
    <div className="page-heading">
      <div>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {extra && <div className="page-heading-extra">{extra}</div>}
    </div>
  );
}
