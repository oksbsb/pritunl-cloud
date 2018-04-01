/// <reference path="../References.d.ts"/>
import * as React from 'react';
import * as CertificateTypes from '../types/CertificateTypes';
import CertificatesStore from '../stores/CertificatesStore';
import * as CertificateActions from '../actions/CertificateActions';
import NonState from './NonState';
import Certificate from './Certificate';
import Page from './Page';
import PageHeader from './PageHeader';

interface State {
	certificates: CertificateTypes.CertificatesRo;
	disabled: boolean;
}

const css = {
	header: {
		marginTop: '-19px',
	} as React.CSSProperties,
	heading: {
		margin: '19px 0 0 0',
	} as React.CSSProperties,
	button: {
		margin: '8px 0 0 8px',
	} as React.CSSProperties,
	buttons: {
		marginTop: '8px',
	} as React.CSSProperties,
	noCerts: {
		height: 'auto',
	} as React.CSSProperties,
};

export default class Certificates extends React.Component<{}, State> {
	constructor(props: any, context: any) {
		super(props, context);
		this.state = {
			certificates: CertificatesStore.certificates,
			disabled: false,
		};
	}

	componentDidMount(): void {
		CertificatesStore.addChangeListener(this.onChange);
		CertificateActions.sync();
	}

	componentWillUnmount(): void {
		CertificatesStore.removeChangeListener(this.onChange);
	}

	onChange = (): void => {
		this.setState({
			...this.state,
			certificates: CertificatesStore.certificates,
		});
	}

	render(): JSX.Element {
		let certsDom: JSX.Element[] = [];

		this.state.certificates.forEach((
				cert: CertificateTypes.CertificateRo): void => {
			certsDom.push(<Certificate
				key={cert.id}
				certificate={cert}
			/>);
		});

		return <Page>
			<PageHeader>
				<div className="layout horizontal wrap" style={css.header}>
					<h2 style={css.heading}>Certificates</h2>
					<div className="flex"/>
					<div style={css.buttons}>
						<button
							className="pt-button pt-intent-success pt-icon-add"
							style={css.button}
							disabled={this.state.disabled}
							type="button"
							onClick={(): void => {
								this.setState({
									...this.state,
									disabled: true,
								});
								CertificateActions.create(null).then((): void => {
									this.setState({
										...this.state,
										disabled: false,
									});
								}).catch((): void => {
									this.setState({
										...this.state,
										disabled: false,
									});
								});
							}}
						>New</button>
					</div>
				</div>
			</PageHeader>
			<div>
				{certsDom}
			</div>
			<NonState
				hidden={!!certsDom.length}
				iconClass="pt-icon-endorsed"
				title="No certificates"
				description="Add a new certificate to get started."
			/>
		</Page>;
	}
}
